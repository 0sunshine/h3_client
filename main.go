package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/quic-go/logging"
	"github.com/quic-go/quic-go/qlog"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
	"log"
	"net/http"
	"os"
	"os/signal"
	"quic_client/utils"
	"syscall"
)

var logFile *lumberjack.Logger

func init_log() {
	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	logFile := &lumberjack.Logger{
		Filename:   "quic_client_log.txt",
		MaxSize:    50, // MB
		MaxBackups: 10,
		MaxAge:     28, // days
		Compress:   false,
	}

	logrus.SetOutput(logFile)
	logrus.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	logrus.SetLevel(logrus.Level(Conf.Log.Level))

	logrus.SetReportCaller(true)
}

func initCert() (*x509.CertPool, error) {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		logrus.Fatal(err)
		return nil, err
	}

	caCertRaw, err := os.ReadFile("./ca.pem")
	if err != nil {
		logrus.Fatal(err)
		return nil, err
	}

	if ok := certPool.AppendCertsFromPEM(caCertRaw); !ok {
		logrus.Fatal("Could not add root ceritificate to pool.")
		return nil, err
	}

	return certPool, nil
}

func waitForQuit() {
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		fmt.Println(sig)
		done <- true
	}()

	<-done

	fmt.Println("quit ....")
}

var h3Client *http.Client = nil

func main() {
	err := LoadConf()
	if err != nil {
		return
	}

	init_log()
	defer func() {
		err := logFile.Close()
		if err != nil {
			fmt.Println(err)
		}
	}()

	certPool, _ := initCert()
	if certPool == nil {
		return
	}

	var qconf quic.Config
	qconf.Tracer = func(ctx context.Context, p logging.Perspective, connID quic.ConnectionID) *logging.ConnectionTracer {
		filename := fmt.Sprintf("client_%s.qlog", connID)
		f, err := os.Create(filename)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Creating qlog file %s.\n", filename)
		return qlog.NewConnectionTracer(utils.NewBufferedWriteCloser(bufio.NewWriter(f), f), p, connID)
	}

	roundTripper := &http3.RoundTripper{
		TLSClientConfig: &tls.Config{
			RootCAs:            certPool,
			InsecureSkipVerify: true,
		},
		QuicConfig: &qconf,
	}

	defer roundTripper.Close()
	h3Client = &http.Client{
		Transport: roundTripper,
	}

	DoSessDispatch()

	//var wg sync.WaitGroup
	//wg.Add(len(urls))
	//
	//for _, addr := range urls {
	//	go func(addr string) {
	//		defer wg.Done()
	//
	//		resp, err := h3Client.Get(addr)
	//		if err != nil {
	//			log.Fatal(err)
	//			return
	//		}
	//
	//		defer resp.Body.Close()
	//
	//		if resp.StatusCode != http.StatusOK {
	//			logrus.Error("Failed to download: ", addr, ", HTTP Status Code: ", resp.StatusCode)
	//			return
	//		}
	//
	//		buf := make([]byte, 1024*64) //64k
	//		for {
	//			_, err := resp.Body.Read(buf)
	//			if err == io.EOF {
	//				break
	//			} else if err != nil {
	//				logrus.Error("Failed to download: ", addr, ", err: ", err)
	//				return
	//			}
	//		}
	//		logrus.Debug("download ok: ", addr)
	//	}(addr)
	//}
	//
	//wg.Wait()

	waitForQuit()
	logrus.Info("exit.......")
}
