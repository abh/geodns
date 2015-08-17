package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

/*
 * Each HealthTester is a struct that implements the HealthTester interface. To do so it
 * needs to provide three functions:
 *
 * func (m *MyHealthTester) Test(ht *HealthTest) bool
 *
 *   performs the test in question and returns a bool if it is up
 *
 * func (m *MyHealthTester) String() string
 *
 *   returns a string which contains all the paramaters within the struct that are important
 *   for uniqueness. This is normally a call to fmt.Sprintf.
 *
 * func newMyHealthTester(params map[string]interface{}) HealthTester, bool
 *
 *   create a new health tester of type myHealthTester with parameters in params. Second
 *   return value true if it is global (i.e. one test yields results for all IP addresses)
 *
 * Then add a single entry to the HealthTesterTypes map pointing to the third function
 */

var HealthTesterMap = map[string]func(params map[string]interface{}, htp *HealthTestParameters) HealthTester{
	"tcp":      newTcpHealthTester,
	"ntp":      newNtpHealthTester,
	"exec":     newExecHealthTester,
	"file":     newFileHealthTester,
	"nodeping": newNodepingHealthTester,
	"pingdom":  newPingdomHealthTester,
}

// TcpHealthTester tests that a port is open
//
// Parameters:
//   port (integer): the port to test

type TcpHealthTester struct {
	port int
}

func (t *TcpHealthTester) Test(ht *HealthTest) bool {
	if conn, err := net.DialTimeout("tcp", net.JoinHostPort(ht.ipAddress.String(), strconv.Itoa(t.port)), ht.timeout); err != nil {
		return false
	} else {
		conn.Close()
	}
	return true
}

func (t *TcpHealthTester) String() string {
	return fmt.Sprintf("%d", t.port)
}

func newTcpHealthTester(params map[string]interface{}, htp *HealthTestParameters) HealthTester {
	port := 80
	if v, ok := params["port"]; ok {
		port = valueToInt(v)
	}
	return &TcpHealthTester{port: port}
}

// NtpHealthTester tests that NTP is running and is less than or equal to a given NTP Stratum
//
// Parameters:
//   max_stratum (integer): the maximum permissible NTP stratum

type NtpHealthTester struct {
	maxStratum int
}

func (t *NtpHealthTester) Test(ht *HealthTest) bool {
	udpAddress, err := net.ResolveUDPAddr("udp", net.JoinHostPort(ht.ipAddress.String(), "123"))
	if err != nil {
		return false
	}

	data := make([]byte, 48)
	data[0] = 4<<3 | 3 /* version 4, client mode */

	conn, err := net.DialUDP("udp", nil, udpAddress)
	if err != nil {
		return false
	}

	defer conn.Close()

	_, err = conn.Write(data)
	if err != nil {
		return false
	}

	conn.SetDeadline(time.Now().Add(ht.timeout))

	_, err = conn.Read(data)
	if err != nil {
		return false
	}

	stratum := data[1]

	if stratum == 0 || stratum > byte(t.maxStratum) {
		return false
	}

	return true
}

func (t *NtpHealthTester) String() string {
	return fmt.Sprintf("%d", t.maxStratum)
}

func newNtpHealthTester(params map[string]interface{}, htp *HealthTestParameters) HealthTester {
	maxStratum := 3
	if v, ok := params["max_stratum"]; ok {
		maxStratum = valueToInt(v)
	}
	return &NtpHealthTester{maxStratum: maxStratum}
}

// ExecHealthTester tests that an external program runs with a zero exit code
//
// Parameters:
//   cmd (string): path to the external program plus space-separated parameters
//
// A {} in the command is substituted with the IP to test

type ExecHealthTester struct {
	cmd string
}

func (t *ExecHealthTester) Test(ht *HealthTest) bool {
	commandSlice := strings.Split(strings.Replace(t.cmd, "{}", ht.ipAddress.String(), -1), " ")
	cmd := exec.Command(commandSlice[0], commandSlice[1:]...)
	return cmd.Run() == nil
}

func (t *ExecHealthTester) String() string {
	return fmt.Sprintf("%s", t.cmd)
}

func newExecHealthTester(params map[string]interface{}, htp *HealthTestParameters) HealthTester {
	cmd := "echo '%s'"
	if v, ok := params["cmd"]; ok {
		cmd = valueToString(v)
	}
	return &ExecHealthTester{cmd: cmd}
}

// FileHealthTester reads health of IP addresses from an external JSON map
//
// Parameters:
//   path (string): path to the JSON file
//
// The JSON file is of the format:
//
//     {
//       "192.200.0.1": true,
//       "192.200.0.2": false
//     }

type FileHealthTester struct {
	path         string
	lastHash     string
	lastReadTime time.Time
}

func (t *FileHealthTester) Test(ht *HealthTest) bool {
	if len(t.path) == 0 {
		logPrintf("No test file path specified")
		return false
	}

	if file, err := os.Open(t.path); err != nil {
		log.Printf("Cannot open test file '%s': %v", t.path, err)
		return false
	} else {
		defer file.Close()
		if stat, err := file.Stat(); err != nil {
			log.Printf("Cannot stat test file '%s': %v", t.path, err)
			return false
		} else {
			modTime := stat.ModTime()
			if modTime == t.lastReadTime {
				return true
			}
			if bytes, err := ioutil.ReadAll(file); err != nil {
				log.Printf("Cannot read test file '%s': %v", t.path, err)
				return false
			} else {
				t.lastReadTime = modTime

				hasher := sha256.New()
				hasher.Write(bytes)
				hash := hex.EncodeToString(hasher.Sum(nil))
				if hash == t.lastHash {
					return true
				}
				t.lastHash = hash

				var m map[string]bool
				if err := json.Unmarshal(bytes, &m); err != nil {
					log.Printf("Cannot parse test file '%s': %v", t.path, err)
					return false
				}
				ht.setGlobal(m)
				return true
			}
		}
	}
	return false
}

func (t *FileHealthTester) String() string {
	return fmt.Sprintf("%s", t.path)
}

func newFileHealthTester(params map[string]interface{}, htp *HealthTestParameters) HealthTester {
	var path string
	if v, ok := params["path"]; ok {
		path = valueToString(v)
	}
	htp.global = true
	return &FileHealthTester{path: path}
}

// NodepingHealthTester reads health of IP addresses from an external Nodeping service
//
// The label of each test must correspond to the IP address being tested
//
// Parameters:
//   token (string): API key token to use with nodeping service
//
// If the token is not specified it defaults to the token within the [nodeping] section of the config file

type NodepingHealthTester struct {
	token string
}

/* Response is of the form below - only down sites are mentioned
{
   "201511111111111-AAAAAAAAA" : {
      "_id" : "201511111111111-AAAAAAAAA-11111111111",
      "label" : "192.200.0.1",
      "type" : "down",
      "message" : "Error: connect ECONNREFUSED",
      "t" : 14141414141414
   }
}
*/

func (t *NodepingHealthTester) Test(ht *HealthTest) bool {
	token := t.token
	if len(token) == 0 {
		cfgMutex.RLock()
		token = Config.Nodeping.Token
		cfgMutex.RUnlock()
		if len(token) == 0 {
			logPrintf("No Nodeping API key specified")
			return false
		}
	}

	var vals url.Values = url.Values{}
	vals.Set("token", token)
	u := url.URL{
		Host:   "api.nodeping.com",
		Scheme: "https",
		Path:   "/api/1/results/current",
	}
	u.RawQuery = vals.Encode()
	if resp, err := http.Get(u.String()); err != nil {
		log.Printf("Cannot access Nodeping API : %v", err)
		return false
	} else {
		defer resp.Body.Close()
		if bytes, err := ioutil.ReadAll(resp.Body); err != nil {
			log.Printf("Cannot read from Nodeping API: %v", err)
			return false
		} else {
			var m map[string]interface{}
			if err := json.Unmarshal(bytes, &m); err != nil {
				log.Printf("Cannot parse response from Nodeping API: %v", err)
				return false
			}

			state := make(map[string]bool)
			for _, item := range m {
				if result, ok := item.(map[string]interface{}); ok {
					if ip, ok := result["label"]; ok {
						host := valueToString(ip)
						logPrintf("Nodeping host %s health set to false", host)
						state[host] = false // only down or disabled events reported
					}
				}
			}

			ht.setGlobal(state)
			return true
		}

	}
	return false
}

func (t *NodepingHealthTester) String() string {
	return fmt.Sprintf("%s", t.token)
}

func newNodepingHealthTester(params map[string]interface{}, htp *HealthTestParameters) HealthTester {
	var token string
	if v, ok := params["token"]; ok {
		token = valueToString(v)
	}
	// as we can only detect down nodes, not all nodes, we should assume the default is health
	htp.healthyInitially = true
	htp.global = true
	return &NodepingHealthTester{token: token}
}

// PingdomHealthTester reads health of IP addresses from an external Pingdom service
//
// The name of each test must correspond to the IP address being tested
//
// Parameters:
//   username (string): username to use with Pingdom service
//   password (string): password to use with the Pingdom service
//   account_email (string, optional): account email to use with Pingdom service (multi user accounts only)
//   app_key (string, optional): application key to use with the Pingdom service (has a sensible default)
//   state_map (map, optional): map of Pingdom status to health values (e.g. true/false)
//
// If any of the above are not specified, they default to the following fields within the [pingdom] section of the config file:
// 		username
//		password
//		accountemail
//		appkey
//		statemap
//
// The stateMap parameter is optional and normally not required. It defaults to:
//    { "up": true, "down": false, "paused": false}
// which means 'up' corresponds to healthy, 'down' and 'paused' to unhealthy, and the remainder to the default value.
//
// To include 'unconfirmed_down' as unhealthy as well, one would use:
//    { "up": true, "down": false, "paused": false, "unconfirmed_down", false}
//

type PingdomHealthTester struct {
	username     string
	password     string
	accountEmail string
	appKey       string
	stateMap     map[string]bool
}

/* Response is of the form below

{
    "checks": [
        {
            "hostname": "example.com",
            "id": 85975,
            "lasterrortime": 1297446423,
            "lastresponsetime": 355,
            "lasttesttime": 1300977363,
            "name": "My check 1",
            "resolution": 1,
            "status": "up",
            "type": "http",
            "tags": [
                {
                    "name": "apache",
                    "type": "a",
                    "count": 2
                }
            ]
        },
        ...
    ]
}
*/

func (t *PingdomHealthTester) Test(ht *HealthTest) bool {
	username := t.username
	if len(username) == 0 {
		cfgMutex.RLock()
		username = Config.Pingdom.Username
		cfgMutex.RUnlock()
		if len(username) == 0 {
			logPrintf("No Pingdom username specified")
			return false
		}
	}

	password := t.password
	if len(password) == 0 {
		cfgMutex.RLock()
		password = Config.Pingdom.Password
		cfgMutex.RUnlock()
		if len(password) == 0 {
			logPrintf("No Pingdom password specified")
			return false
		}
	}

	accountEmail := t.accountEmail
	if len(accountEmail) == 0 {
		cfgMutex.RLock()
		accountEmail = Config.Pingdom.AccountEmail
		cfgMutex.RUnlock()
	}

	appKey := t.appKey
	if len(appKey) == 0 {
		cfgMutex.RLock()
		appKey = Config.Pingdom.AppKey
		cfgMutex.RUnlock()
		if len(appKey) == 0 {
			appKey = "gyxtnd2fzco8ys29m8luk4syag4ybmc0"
		}
	}

	stateMap := t.stateMap
	if stateMap == nil {
		cfgMutex.RLock()
		stateMapString := Config.Pingdom.StateMap
		cfgMutex.RUnlock()
		if len(stateMapString) > 0 {
			stateMap = make(map[string]bool)
			if err := json.Unmarshal([]byte(stateMapString), &stateMap); err != nil {
				logPrintf("Cannot decode configfile Pingdom state map JSON")
				return false
			}
		}
		if stateMap == nil {
			stateMap = defaultPingdomStateMap
		}
	}

	var vals url.Values = url.Values{}
	u := url.URL{
		Host:   "api.pingdom.com",
		Scheme: "https",
		Path:   "/api/2.0/checks",
	}
	u.RawQuery = vals.Encode()

	client := &http.Client{}

	if req, err := http.NewRequest("GET", u.String(), nil); err != nil {
		log.Printf("Cannot construct Pingdom API request: %v", err)
	} else {
		req.SetBasicAuth(username, password)
		if len(accountEmail) > 0 {
			req.Header.Add("Account-Email", accountEmail)
		}
		req.Header.Add("App-Key", appKey)
		if resp, err := client.Do(req); err != nil {
			log.Printf("Cannot access Pingdom API : %v", err)
			return false
		} else {
			defer resp.Body.Close()
			if bytes, err := ioutil.ReadAll(resp.Body); err != nil {
				log.Printf("Cannot read from Pingdom API: %v", err)
				return false
			} else {
				var m map[string]interface{}
				if err := json.Unmarshal(bytes, &m); err != nil {
					log.Printf("Cannot parse response from Pingdom API: %v", err)
					return false
				}
				if checks, ok := m["checks"]; !ok {
					log.Printf("Cannot parse response from Pingdom API check response")
					return false
				} else {
					if checkarray, ok := checks.([]interface{}); !ok {
						log.Printf("Cannot parse response from Pingdom API check array: %T", checks)
						return false
					} else {
						state := make(map[string]bool)
						for _, checki := range checkarray {
							if check, ok := checki.(map[string]interface{}); ok {
								if ip, ok := check["name"]; ok {
									if status, ok := check["status"]; ok {
										s := valueToString(status)
										if updown, ok := stateMap[s]; ok {
											host := valueToString(ip)
											state[host] = updown
											logPrintf("Pingdom host %s state %s health set to %v", host, s, updown)
										}
									}
								}
							}
						}

						ht.setGlobal(state)
						return true
					}
				}
			}
		}
	}
	return false
}

func (t *PingdomHealthTester) String() string {
	return fmt.Sprintf("%s/%s/%s/%s/%v", t.username, t.password, t.accountEmail, t.appKey, t.stateMap)
}

var defaultPingdomStateMap = map[string]bool{
	"up":     true,
	"down":   false,
	"paused": false,
	// other states, i.e. unconfirmed_down, paused, are determined by initially_healthy
}

func newPingdomHealthTester(params map[string]interface{}, htp *HealthTestParameters) HealthTester {
	var username string
	var password string
	var accountEmail string
	var appKey string
	var stateMap map[string]bool = nil
	if v, ok := params["username"]; ok {
		username = valueToString(v)
	}
	if v, ok := params["password"]; ok {
		password = valueToString(v)
	}
	if v, ok := params["account_email"]; ok {
		accountEmail = valueToString(v)
	}
	if v, ok := params["app_key"]; ok {
		appKey = valueToString(v)
	}
	if v, ok := params["state_map"]; ok {
		if vv, ok := v.(map[string]interface{}); ok {
			stateMap = make(map[string]bool)
			for k, s := range vv {
				stateMap[valueToString(k)] = valueToBool(s)
			}
		}
	}
	htp.global = true
	return &PingdomHealthTester{username: username, password: password, accountEmail: accountEmail, appKey: appKey, stateMap: stateMap}
}
