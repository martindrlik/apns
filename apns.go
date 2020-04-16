package apns

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/martindrlik/org/confirm"
	"go.uber.org/zap"
	"golang.org/x/crypto/pkcs12"
)

type ignoreHTTPWriter struct {
	logger *zap.SugaredLogger
}

func (fw *ignoreHTTPWriter) Write(p []byte) (n int, err error) {
	if !strings.HasPrefix(string(p), "http:") {
		fw.logger.Errorw(string(p))
		return len(p), nil
	}
	return len(p), nil
}

func iosHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	dec := json.NewDecoder(r.Body)

	var notification notification
	err := dec.Decode(&notification)
	if err != nil {
		panic(err)
	}
	log.Println("AppId from body: ", notification.ApplicationID)
	log.Println("Host: ", notification.BaseURL)
	log.Println("NotifToken: ", notification.NotificationToken)

	confirmrequest := confirmrequest{notification.ApplicationID, notification.NotificationToken, "Ios"}

	if isSuccessCode(notification.ResponseCode) &&
		notification.ResponseError == "" {
		confirm.Channel <- confirm.Payload{
			ApplicationId: confirmrequest.ApplicationID,
			BaseURL:       notification.BaseURL,
			Platform:      confirmrequest.Platform,
			Token:         confirmrequest.NotificationToken}
	}
	if notification.ResponseCode != 0 {
		w.WriteHeader(notification.ResponseCode)
	}
	if notification.ResponseError != "" {
		enc := json.NewEncoder(w)
		if err := enc.Encode(response{
			Reason: notification.ResponseError}); err != nil {
			log.Fatal(err)
		}
	}
}

func isSuccessCode(i int) bool {
	return i >= 200 && i <= 300
}

type notification struct {
	NotificationToken string
	BaseURL           string
	ApplicationID     uint64
	ResponseCode      int
	ResponseError     string
}

type confirmrequest struct {
	ApplicationID     uint64
	NotificationToken string
	Platform          string
}

func mustDecodeCert(_name, password string) *x509.Certificate {
	bytes, err := ioutil.ReadFile(_name)
	if err != nil {
		log.Fatal(err)
	}

	_, cert, err := pkcs12.Decode(bytes, password)
	if err != nil {
		log.Fatal(err)
	}
	return cert
}

// ListenAndServeTLS always returns a non-nil error. After Shutdown or
// Close, the returned error is ErrServerClosed.
func ListenAndServeTLS(addr, certFile, keyFile, appleCert, password string) error {
	flag.Parse()
	mux := &http.ServeMux{}
	mux.HandleFunc("/3/device/", iosHandler)

	pool := x509.NewCertPool()
	pool.AddCert(mustDecodeCert(appleCert, password))

	logger := zap.NewExample()
	defer logger.Sync()
	sugar := logger.Sugar()

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
		TLSConfig: &tls.Config{
			ClientAuth: tls.RequireAndVerifyClientCert,
			ClientCAs:  pool,
		},
		//ErrorLog: log.New(ioutil.Discard, "", 0),
		ErrorLog: log.New(&ignoreHTTPWriter{sugar}, "", 0),
	}
	return server.ListenAndServeTLS(certFile, keyFile)
}

type response struct {
	Reason string
}
