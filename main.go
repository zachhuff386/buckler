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
	wsReplacer    = strings.NewReplacer("__", "_", "_", " ")
	revWsReplacer = strings.NewReplacer(" ", "_", "_", "__", "-", "--")

	// set last modifed to server startup. close enough to release.
	lastModified    = time.Now()
	lastModifiedStr = lastModified.UTC().Format(http.TimeFormat)
	oneYear         = time.Duration(8700) * time.Hour

	staticPath, _ = resourcePaths()

	redisAddress = "localhost:6379"
	pypiExpireTime = "3600"
	droneExpireTime = "60"

	verQuery = "version"
	dayDownQuery = "day_down"
	weekDownQuery = "week_down"
	monthDownQuery = "month_down"
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

func queryPypi(project string, query string) (value string, err error) {
	conn, err := redis.Dial("tcp", redisAddress)
	if err != nil {
		return
	}
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

	conn.Close()
	return
}

func queryDrone(owner string, project string) (status string, err error) {
	conn, err := redis.Dial("tcp", redisAddress)
	if err != nil {
		return
	}
	redisKey := owner + "_" + project + "_drone"

	status, e := redis.String(conn.Do("GET", redisKey))
	if e != nil {
		resp, e := http.Get(
			"https://drone.io/github.com/" + owner + "/" + project + "/status.png")
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

	conn.Close()
	return
}

func praseParts(parts []string) (cache bool, data Data, err error) {
	if len(parts) < 6 {
		err = errors.New("Query invalid")
		return
	}

	if !strings.HasSuffix(parts[5], ".png") {
		err = errors.New("Unknown file type")
		return
	}

	var pypiValue string
	shieldType := parts[2]
	key := parts[3]
	value := parts[4]
	color := parts[5][0:len(parts[5]) - 4]
	cache = false

	switch shieldType {
	case "text":
		cache = true
	case "pypi":
		pypiValue, err = queryPypi(key, value)
		if err != nil {
			return
		}

		switch value {
		case dayDownQuery:
			pypiValue += " today"
		case weekDownQuery:
			pypiValue += " this week"
		case monthDownQuery:
			pypiValue += " this month"
		}

		if value == verQuery {
			key = "version"
		} else {
			key = "downloads"
		}
		value = pypiValue
	case "drone":
		value, err = queryDrone(key, value)
		if err != nil {
			return
		}

		colors := strings.Split(color, "-")
		if len(colors) != 2 {
			err = errors.New("Shield color invalid")
			return
		}

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

func buckle(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")

	c, d, err := praseParts(parts)
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

const basePkg = "github.com/badges/buckler"

func index(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join(staticPath, "index.html"))
}

func favicon(w http.ResponseWriter, r *http.Request) {
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
	portEnv := os.Getenv("PORT")

	// default to environment variable values (changes the help string :( )
	if hostEnv == "" {
		hostEnv = "*"
	}

	p := 8080
	if portEnv != "" {
		p, _ = strconv.Atoi(portEnv)
	}

	goopt.Usage = usage

	// server mode options
	host := goopt.String([]string{"-h", "--host"}, hostEnv, "host ip address to bind to")
	port := goopt.Int([]string{"-p", "--port"}, p, "port to listen on")
	redisAddressOpt := goopt.String([]string{"-r", "--redis"}, redisAddress, "redis server address")
	goopt.Parse(nil)

	redisAddress = *redisAddressOpt

	// normalize for http serving
	if *host == "*" {
		*host = ""
	}

	http.HandleFunc("/v1/", buckle)
	http.HandleFunc("/favicon.png", favicon)
	http.HandleFunc("/", index)

	log.Println("Listening on port", *port)
	http.ListenAndServe(*host+":"+strconv.Itoa(*port), nil)
}
