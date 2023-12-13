package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/quic-go/logging"
	"github.com/quic-go/quic-go/qlog"
	"github.com/sirupsen/logrus"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"quic_client/utils"
	"strconv"
	"sync"
	"time"
)

type SessDispatch struct {
	playConf      PlayYamlConf
	playList      []string
	currIdx       int
	playListMutex sync.Mutex
}

func (this *SessDispatch) getPlayList() {
	f, err := os.Open(this.playConf.PlayList)
	if err != nil {
		logrus.Error(err)
		return
	}

	this.playList = []string{}

	r := bufio.NewReader(f)
	for {
		bytes, _, err := r.ReadLine()
		if err == io.EOF {
			break
		} else if err != nil {
			logrus.Error(err)
			return
		}

		s := string(bytes)
		if len(s) > 0 {
			this.playList = append(this.playList, string(bytes))
		}
	}
}

func (this *SessDispatch) getUrlFromPlayList() (string, error) {

	this.playListMutex.Lock()
	defer this.playListMutex.Unlock()

	url := ""
	useIdx := 0

	switch this.playConf.PlayListSelectType {
	case 0:
		{ //顺序]
			useIdx = this.currIdx
			if this.currIdx >= (len(this.playList) - 1) {
				this.currIdx = 0
			} else {
				this.currIdx++
			}

			break
		}

	case 1:
		{ //随机
			useIdx = rand.Int() % len(this.playList)
		}

	}

	url = this.playList[useIdx]

	if len(url) == 0 {
		return "", errors.New("no playList")
	}

	return url, nil
}

func (this *SessDispatch) do() {
	this.getPlayList()
	for i := 0; i < this.playConf.SessMax; i++ {

		if i > this.playConf.SessMin {
			time.Sleep(time.Duration(this.playConf.SessIncreaseSpeed) * time.Millisecond)
		}

		s := NewSession(strconv.Itoa(i), this, this.playConf.SessBytesPerSec, this.playConf.SessRepeat, &WorkLimiter{
			SessContinuousPlayTime: this.playConf.SessContinuousPlayTime,
			SessPauseTime:          this.playConf.SessPauseTime,
		})

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

		s.httpClient = &http.Client{
			Transport: roundTripper,
		}

		go func() {
			defer roundTripper.Close()
			s.Do()
		}()
	}
}

func DoSessDispatch() {
	for _, v := range Conf.Play {
		if len(v.PlayList) == 0 {
			continue
		}

		sess := SessDispatch{
			playConf: v,
		}

		go func() {
			sess.do()
		}()
	}
}
