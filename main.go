package main

import (
	"crypto/x509"
	"fmt"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"os/signal"
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

var certPool *x509.CertPool = nil

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

	DoSessDispatch()

	waitForQuit()
	logrus.Info("exit.......")
}
