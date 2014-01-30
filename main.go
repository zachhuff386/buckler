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
	redisVerField = "version"
	redisDayDownField = "ddownloads"
	redisWeekDownField = "wdownloads"
	redisMonDownField = "mdownloads"
	pypiExpireTime = "3600"
	droneExpireTime = "300"
)

func shift(s []string) ([]string, string) {
	return s[1:], s[0]
}

func invalidRequest(w http.ResponseWriter, r *http.Request) {
	log.Println("bad request", r.URL.String())
	http.Error(w, "bad request", 400)
}

func parseFileName(name string) (d Data, err error) {
	imageName := wsReplacer.Replace(name)
	imageParts := strings.Split(imageName, "-")

	newParts := []string{}
	for len(imageParts) > 0 {
		var head, right string
		imageParts, head = shift(imageParts)

		// if starts with - append to previous
		if len(head) == 0 && len(newParts) > 0 {
			left := ""
			if len(newParts) > 0 {
				left = newParts[len(newParts)-1]
				newParts = newParts[:len(newParts)-1]
			}

			// trailing -- is going to break color anyways so don't worry
			imageParts, right = shift(imageParts)

			head = strings.Join([]string{left, right}, "-")
		}

		newParts = append(newParts, head)
	}

	if len(newParts) != 3 {
		err = errors.New("Invalid file name")
		return
	}

	if !strings.HasSuffix(newParts[2], ".png") {
		err = errors.New("Unknown file type")
		return
	}

	cp := newParts[2][0 : len(newParts[2])-4]
	c, err := getColor(cp)
	if err != nil {
		return
	}

	d = Data{newParts[0], newParts[1], c}
	return
}

func formatNum(num int) (formated string) {
	if num >= 1000000 {
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
		conn.Send("HSET", redisKey, redisVerField, version)
		conn.Send("HSET", redisKey, redisDayDownField, downloadsDay)
		conn.Send("HSET", redisKey, redisWeekDownField, downloadsWeek)
		conn.Send("HSET", redisKey, redisMonDownField, downloadsMon)
		conn.Send("EXPIRE", redisKey, pypiExpireTime)
		_, e = conn.Do("EXEC")
		if e != nil {
			return value, e
		}

		switch query {
		case redisVerField:
			value = version
		case redisDayDownField:
			value = downloadsDay
		case redisWeekDownField:
			value = downloadsWeek
		case redisMonDownField:
			value = downloadsMon
		}
	}

	conn.Close()
	return
}

func queryDrone(project string) (status string, err error) {
	conn, err := redis.Dial("tcp", redisAddress)
	if err != nil {
		return
	}
	redisKey := project + "_drone"

	status, e := redis.String(conn.Do("GET", redisKey))
	if e != nil {
		resp, e := http.Get(
			"https://drone.io/github.com/" + project + "/status.png")
		if e != nil {
			return status, e
		}
		defer resp.Body.Close()
		body, e := ioutil.ReadAll(resp.Body)

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

func parseFileNameDrone(name string) (d Data, err error) {
	imageName := wsReplacer.Replace(name)
	imageParts := strings.Split(imageName, "-")

	newParts := []string{}
	for len(imageParts) > 0 {
		var head, right string
		imageParts, head = shift(imageParts)

		// if starts with - append to previous
		if len(head) == 0 && len(newParts) > 0 {
			left := ""
			if len(newParts) > 0 {
				left = newParts[len(newParts)-1]
				newParts = newParts[:len(newParts)-1]
			}

			// trailing -- is going to break color anyways so don't worry
			imageParts, right = shift(imageParts)

			head = strings.Join([]string{left, right}, "-")
		}

		newParts = append(newParts, head)
	}

	if len(newParts) != 4 {
		err = errors.New("Invalid file name")
		return
	}

	if !strings.HasSuffix(newParts[3], ".png") {
		err = errors.New("Unknown file type")
		return
	}

	status, err := queryDrone(newParts[0] + "/" + newParts[1])
	if err != nil {
		return
	}
	var cp string
	if status == "passing" {
		cp = newParts[2]
	} else {
		cp = newParts[3][0 : len(newParts[3])-4]
	}

	c, err := getColor(cp)
	if err != nil {
		return
	}

	d = Data{"build", status, c}
	return
}

func parseFileNamePypi(name string) (d Data, err error) {
	imageName := wsReplacer.Replace(name)
	imageParts := strings.Split(imageName, "-")

	newParts := []string{}
	for len(imageParts) > 0 {
		var head, right string
		imageParts, head = shift(imageParts)

		// if starts with - append to previous
		if len(head) == 0 && len(newParts) > 0 {
			left := ""
			if len(newParts) > 0 {
				left = newParts[len(newParts)-1]
				newParts = newParts[:len(newParts)-1]
			}

			// trailing -- is going to break color anyways so don't worry
			imageParts, right = shift(imageParts)

			head = strings.Join([]string{left, right}, "-")
		}

		newParts = append(newParts, head)
	}

	if len(newParts) != 3 {
		err = errors.New("Invalid file name")
		return
	}

	if !strings.HasSuffix(newParts[2], ".png") {
		err = errors.New("Unknown file type")
		return
	}

	cp := newParts[2][0 : len(newParts[2])-4]
	c, err := getColor(cp)
	if err != nil {
		return
	}

	var query string
	switch newParts[1] {
	case "ver":
		query = redisVerField
	case "ddl":
		query = redisDayDownField
	case "wdl":
		query = redisWeekDownField
	case "mdl":
		query = redisMonDownField
	default:
		err = errors.New("Unknown pypi query")
		return
	}

	value, err := queryPypi(newParts[0], query)
	if err != nil {
		return
	}

	var key string
	if query == redisVerField {
		key = "version"
	} else {
		key = "downloads"
		switch query {
		case redisDayDownField:
			value += " today"
		case redisWeekDownField:
			value += " this week"
		case redisMonDownField:
			value += " this month"
		}
	}

	d = Data{key, value, c}
	return
}

func buckle(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")

	parts_len := len(parts)
	if parts_len < 3 || parts_len > 4 {
		invalidRequest(w, r)
		return
	}

	var d Data
	var err error
	if parts[2] == "pypi" {
		d, err = parseFileNamePypi(parts[3])
	} else if parts[2] == "drone" {
		d, err = parseFileNameDrone(parts[3])
	} else {
		d, err = parseFileName(parts[2])
	}
	if err != nil {
		invalidRequest(w, r)
		return
	}

	t, err := time.Parse(time.RFC1123, r.Header.Get("if-modified-since"))
	if err == nil && lastModified.Before(t.Add(1*time.Second)) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Add("content-type", "image/png")
	w.Header().Add("expires", time.Now().Add(oneYear).Format(time.RFC1123))
	w.Header().Add("cache-control", "public")
	w.Header().Add("last-modified", lastModifiedStr)

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

func cliMode(vendor string, status string, color string, args []string) {
	// if any of vendor, status or color is given, all must be
	if (vendor != "" || status != "" || color != "") &&
		!(vendor != "" && status != "" && color != "") {
		fatal("You must specify all of vendor, status, and color")
	}

	if vendor != "" {
		c, err := getColor(color)
		if err != nil {
			fatal("Invalid color: " + color)
		}
		d := Data{vendor, status, c}

		name := fmt.Sprintf("%s-%s-%s.png", revWsReplacer.Replace(vendor),
			revWsReplacer.Replace(status), color)

		if len(args) > 1 {
			fatal("You can only specify one output file name")
		}

		if len(args) == 1 {
			name = args[0]
		}

		// default to standard out
		f := os.Stdout
		if name != "-" {
			f, err = os.Create(name)
			if err != nil {
				fatal("Unable to create file: " + name)
			}
		}

		makePngShield(f, d)
		return
	}

	// generate based on command line file names
	for i := range args {
		name := args[i]
		d, err := parseFileName(name)
		if err != nil {
			fatal(err.Error())
		}

		f, err := os.Create(name)
		if err != nil {
			fatal(err.Error())
		}
		makePngShield(f, d)
	}
}

func usage() string {
	u := `Usage: %s [-h HOST] [-p PORT]
       %s [-v VENDOR -s STATUS -c COLOR] <FILENAME>

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

	// cli mode
	vendor := goopt.String([]string{"-v", "--vendor"}, "", "vendor for cli generation")
	status := goopt.String([]string{"-s", "--status"}, "", "status for cli generation")
	color := goopt.String([]string{"-c", "--color", "--colour"}, "", "color for cli generation")

	// redis options
	redisAddressOpt := goopt.String([]string{"-r", "--redis"}, redisAddress, "redis server address")
	goopt.Parse(nil)

	redisAddress = *redisAddressOpt

	args := goopt.Args

	// if any of the cli args are given, or positional args remain, assume cli
	// mode.
	if len(args) > 0 || *vendor != "" || *status != "" || *color != "" {
		cliMode(*vendor, *status, *color, args)
		return
	}
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
