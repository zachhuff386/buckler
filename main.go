package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"io/ioutil"
	"encoding/hex"
	"encoding/json"
	"crypto/md5"
	"bytes"

	"github.com/droundy/goopt"
	"github.com/garyburd/redigo/redis"
)

var (
	// set last modifed to server startup. close enough to release.
	lastModified    = time.Now()
	lastModifiedStr = lastModified.UTC().Format(http.TimeFormat)
	oneYear         = time.Duration(8700) * time.Hour

	staticPath, _ = resourcePaths()

	host = "*"
	port = 8080
	redisAddress = "localhost:6379"
	redisPassword = ""
	pypiExpireTime = "3600"
	droneExpireTime = "60"

	verQuery = "version"
	dayDownQuery = "day_down"
	weekDownQuery = "week_down"
	monthDownQuery = "month_down"
	redisPool redis.Pool
)

func invalidRequest(w http.ResponseWriter, r *http.Request) {
	log.Println("bad request", r.URL.String())
	http.Error(w, "bad request", 400)
}

func formatNum(num int) (formated string) {
	if num >= 1000000000 {
		formated = strconv.FormatFloat(float64(num) / 1000000000, 'f', 1, 64) + "B"
	} else if num >= 1000000 {
		formated = strconv.FormatFloat(float64(num) / 1000000, 'f', 1, 64) + "M"
	} else if num >= 1000 {
		formated = strconv.FormatFloat(float64(num) / 1000, 'f', 1, 64) + "K"
	} else {
		formated = strconv.Itoa(num)
	}
	return
}

func queryPypi(conn redis.Conn, project string, query string) (value string, err error) {
	redisKey := project + "_pypi"

	value, e := redis.String(conn.Do("HGET", redisKey, query))
	if e != nil {
		resp, e := http.Get(
			"http://pypi.python.org/pypi/" + project + "/json")
		if e != nil {
			return value, e
		}

		defer resp.Body.Close()
		body, e := ioutil.ReadAll(resp.Body)
		if e != nil {
			return value, e
		}

		var data interface{}
		json.Unmarshal(body, &data)
		dataMap := data.(map[string]interface{})
		infoMap := dataMap["info"].(map[string]interface{})
		downloadsMap := infoMap["downloads"].(map[string]interface{})
		version := infoMap["version"].(string)
		downloadsDay := formatNum(int(downloadsMap["last_day"].(float64)))
		downloadsWeek := formatNum(int(downloadsMap["last_week"].(float64)))
		downloadsMon := formatNum(int(downloadsMap["last_month"].(float64)))

		conn.Send("MULTI")
		conn.Send("HSET", redisKey, verQuery, version)
		conn.Send("HSET", redisKey, dayDownQuery, downloadsDay)
		conn.Send("HSET", redisKey, weekDownQuery, downloadsWeek)
		conn.Send("HSET", redisKey, monthDownQuery, downloadsMon)
		conn.Send("EXPIRE", redisKey, pypiExpireTime)
		_, e = conn.Do("EXEC")
		if e != nil {
			return value, e
		}

		switch query {
		case verQuery:
			value = version
		case dayDownQuery:
			value = downloadsDay
		case weekDownQuery:
			value = downloadsWeek
		case monthDownQuery:
			value = downloadsMon
		}
	}
	return
}

func queryDrone(conn redis.Conn, project []string) (status string, err error) {
	redisKey := strings.Join(project, "_") + "_drone"

	status, e := redis.String(conn.Do("GET", redisKey))
	if e != nil {
		resp, e := http.Get("https://drone.io/" + strings.Join(project, "/") + "/status.png")
		if e != nil {
			return status, e
		}

		defer resp.Body.Close()
		body, e := ioutil.ReadAll(resp.Body)
		if e != nil {
			return status, e
		}

		var buffer bytes.Buffer
		hash := md5.New()
		buffer.Write(body)
		buffer.WriteTo(hash)
		hexHash := hex.EncodeToString(hash.Sum(nil))

		// passing - 0bfc124d002aa2eac36bf8e5c518c438
		// failing - d8fd5ef8c156955e1c414a752658544a
		if hexHash == "0bfc124d002aa2eac36bf8e5c518c438" {
			status = "passing"
		} else {
			status = "failing"
		}

		conn.Send("MULTI")
		conn.Send("SET", redisKey, status)
		conn.Send("EXPIRE", redisKey, droneExpireTime)
		_, e = conn.Do("EXEC")
		if e != nil {
			return status, e
		}
	}
	return
}

func praseParts(conn redis.Conn, parts []string) (cache bool, data Data, err error) {
	parts_len := len(parts)
	if parts_len < 6 {
		err = errors.New("Query invalid")
		return
	}

	if !strings.HasSuffix(parts[parts_len - 1], ".png") {
		err = errors.New("Unknown file type")
		return
	}

	var key string
	var value string
	shieldType := parts[2]
	color := parts[parts_len - 1][0:len(parts[parts_len - 1]) - 4]
	cache = false

	switch shieldType {
	case "text":
		if parts_len != 6 {
			err = errors.New("Query invalid")
			return
		}

		cache = true
		key = parts[3]
		value = parts[4]
	case "pypi":
		if parts_len != 6 {
			err = errors.New("Query invalid")
			return
		}

		value, err = queryPypi(conn, parts[3], parts[4])
		if err != nil {
			return
		}

		switch parts[4] {
		case dayDownQuery:
			value += " today"
		case weekDownQuery:
			value += " this week"
		case monthDownQuery:
			value += " this month"
		}

		if parts[4] == verQuery {
			key = "version"
		} else {
			key = "downloads"
		}
	case "drone":
		value, err = queryDrone(conn, parts[3:parts_len - 1])
		if err != nil {
			return
		}

		colors := strings.Split(color, "-")
		if len(colors) != 2 {
			err = errors.New("Shield color invalid")
			return
		}

		key = "build"
		if value == "passing" {
			color = colors[0]
		} else {
			color = colors[1]
		}
	default:
		err = errors.New("Unknown shield type")
		return
	}

	colorRgba, err := getColor(color)
	if err != nil {
		return
	}

	data = Data{key, value, colorRgba}
	return
}

func limit(conn redis.Conn, remoteAddr string) (err error) {
	remoteAddr = remoteAddr[:strings.LastIndex(remoteAddr, ":")]
	redisKey := "ip_" + remoteAddr
	redisKeyLong := "ip_" + remoteAddr + "_long"

	count, err := redis.Int(conn.Do("LLEN", redisKeyLong))
	if err != nil {
		return
	} else if count > 100 {
		err = errors.New("Too many requests")
		return
	} else if count == 99 {
		log.Println("too many requests", remoteAddr)
		conn.Send("SADD", "dos", remoteAddr)
	}

	count, err = redis.Int(conn.Do("LLEN", redisKey))
	if err != nil {
		return
	} else if count > 10 {
		err = errors.New("Too many requests")
		return
	}

	redisKeyEx, err := redis.Bool(conn.Do("EXISTS", redisKey))
	if err != nil {
		return
	}
	redisKeyLongEx, err := redis.Bool(conn.Do("EXISTS", redisKeyLong))
	if err != nil {
		return
	}

	conn.Send("MULTI")
	if redisKeyEx {
		conn.Send("RPUSHX", redisKey, "")
	} else {
		conn.Send("RPUSH", redisKey, "")
		conn.Send("EXPIRE", redisKey, 1)
	}
	if redisKeyLongEx {
		conn.Send("RPUSHX", redisKeyLong, "")
	} else {
		conn.Send("RPUSH", redisKeyLong, "")
		conn.Send("EXPIRE", redisKeyLong, 120)
	}
	_, err = conn.Do("EXEC")
	if err != nil {
		return
	}
	return
}

func buckle(w http.ResponseWriter, r *http.Request) {
	conn := redisPool.Get()
	defer conn.Close()

	err := limit(conn, r.RemoteAddr)
	if err != nil {
		http.Error(w, "too many requests", 429)
		return
	}

	parts := strings.Split(r.URL.Path, "/")

	c, d, err := praseParts(conn, parts)
	if err != nil {
		invalidRequest(w, r)
		return
	}

	w.Header().Add("Content-Type", "image/png")
	if c {
		w.Header().Add("Expires", time.Now().Add(oneYear).Format(time.RFC1123))
		w.Header().Add("Cache-Control", "public")
		w.Header().Add("Last-Modified", lastModifiedStr)
	} else {
		w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Add("Pragma", "no-cache")
	}

	makePngShield(w, d)
}

const basePkg = "github.com/zachhuff386/git-shields"

func index(w http.ResponseWriter, r *http.Request) {
	conn := redisPool.Get()
	defer conn.Close()

	err := limit(conn, r.RemoteAddr)
	if err != nil {
		http.Error(w, "too many requests", 429)
		return
	}

	http.ServeFile(w, r, filepath.Join(staticPath, "index.html"))
}

func favicon(w http.ResponseWriter, r *http.Request) {
	conn := redisPool.Get()
	defer conn.Close()

	err := limit(conn, r.RemoteAddr)
	if err != nil {
		http.Error(w, "too many requests", 429)
		return
	}

	http.ServeFile(w, r, filepath.Join(staticPath, "favicon.png"))
}

func fatal(msg string) {
	fmt.Println(msg)
	os.Exit(1)
}

func usage() string {
	u := `Usage: %s [-h HOST] [-p PORT]

%s`
	return fmt.Sprintf(u, os.Args[0], os.Args[0], goopt.Help())
}

func main() {
	hostEnv := os.Getenv("HOST")
	if hostEnv != "" {
		host = hostEnv
	}

	portEnv := os.Getenv("PORT")
	if portEnv != "" {
		port, _ = strconv.Atoi(portEnv)
	}

	redisAddressEnv := os.Getenv("REDIS_ADDR")
	if redisAddressEnv != "" {
		redisAddress = redisAddressEnv
	}

	redisPasswordEnv := os.Getenv("REDIS_PASS")
	if redisPasswordEnv != "" {
		redisPassword = redisPasswordEnv
	}

	goopt.Usage = usage

	host := goopt.String([]string{"-h", "--host"}, host, "host ip address to bind to")
	port := goopt.Int([]string{"-p", "--port"}, port, "port to listen on")
	redisAddress := goopt.String([]string{"-r", "--redis"}, redisAddress, "redis server address")
	redisPassword := goopt.String([]string{"-a", "--redis-pass"}, redisPassword, "redis server password")
	goopt.Parse(nil)

	redisPool = redis.Pool{
		MaxIdle: 6,
		IdleTimeout: 240 * time.Second,
		Dial: func() (conn redis.Conn, err error) {
			conn, err = redis.Dial("tcp", *redisAddress)
			if err != nil {
				return nil, err
			}
			if *redisPassword != "" {
				if _, err := conn.Do("AUTH", *redisPassword); err != nil {
					conn.Close()
					return nil, err
				}
			}
			return
		},
		TestOnBorrow: func(conn redis.Conn, connTime time.Time) (err error) {
			_, err = conn.Do("PING")
			return
		},
	}

	// normalize for http serving
	if *host == "*" {
		*host = ""
	}

	http.HandleFunc("/v2/", buckle)
	http.HandleFunc("/favicon.png", favicon)
	http.HandleFunc("/", index)

	log.Println("Listening on port", *port)
	http.ListenAndServe(*host + ":" + strconv.Itoa(*port), nil)
}
