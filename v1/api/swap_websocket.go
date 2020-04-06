package api

import (
	"coinest/v1/config"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/gorilla/websocket"
	"net"
	"strconv"
	"time"
)

type swapWebsocket struct {
	conn *websocket.Conn
	// table => messages
	tbms map[string]chan []byte
	cap  int
}

func NewSwapWebsocket(cap int) *swapWebsocket {
	w := new(swapWebsocket)
	w.cap = cap
	w.tbms = make(map[string]chan []byte)
	err := w.connect()
	if err != nil {
		glog.Fatal("websocket connect server failed! ", err)
	}
	return w
}

func (w *swapWebsocket) Login() error {
	glog.Infoln("Login...")
	timestamp := time.Now().In(time.UTC).Unix()
	err := w.conn.WriteJSON(map[string]interface{}{
		"op":   "login",
		"args": []string{config.ApiKey, config.Passphrase, strconv.FormatInt(timestamp, 10), accessSign(strconv.FormatInt(timestamp, 10), "", "", "")},
	})
	if err != nil {
		return fmt.Errorf("conn.WriteJSON failed! err:%w", err)
	}
	_, message, err := w.conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("conn.ReadMessage failed! err:%w", err)
	}
	message = decompress(message)
	result := make(map[string]interface{}, 0)
	err = json.Unmarshal(message, &result)
	if err != nil {
		return fmt.Errorf("json.Unmarshal failed! err:%w message:%s", err, string(message))
	}
	//{"event":"login","success":true}
	if result["event"] != "login" || result["success"] != true {
		return fmt.Errorf("login failed! message:%s", string(message))
	}
	return nil
}

func (w *swapWebsocket) Sub(channelTables map[string]string) error {
	glog.Infoln("Sub...", channelTables)
	//{"op": "subscribe", "args": ["swap/ticker:BTC-USD-SWAP", "swap/candle60s:BTC-USD-SWAP"]}

	channels := make([]string, 0, len(channelTables))
	for channel, table := range channelTables {
		channels = append(channels, channel)
		w.tbms[table] = make(chan []byte, w.cap)
	}
	err := w.conn.WriteJSON(map[string]interface{}{
		"op":   "subscribe",
		"args": channels,
	})
	if err != nil {
		return fmt.Errorf("conn.WriteJSON failed! err:%w", err)
	}
	for i := 0; i < len(channels); i++ {
		_, message, err := w.conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("conn.ReadMessage failed! err:%w", err)
		}
		message = decompress(message)
		result := make(map[string]interface{}, 0)
		err = json.Unmarshal(message, &result)
		if err != nil {
			return fmt.Errorf("json.Unmarshal failed! err:%w", err)
		}
		if result["event"] != "subscribe" || result["channel"] != channels[i] {
			return fmt.Errorf("subscribe failed! message:%s", string(message))
		}
	}
	return nil
}

func (w *swapWebsocket) Channel(table string) (<-chan []byte, bool) {
	ch, ok := w.tbms[table]
	return ch, ok
}

func (w *swapWebsocket) Run() {
	glog.Infoln("start run...")
	if w.conn == nil {
		panic("websocket conn is nil")
	}
	keepalive := make(chan int, 128)
	reconnect := func() {
		w.conn.Close()
		for {
			if err := w.connect(); err != nil {
				glog.Warningln("reconnect server failed!", err)
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return
		}
	}

	go func() {
		last := time.Now()
		for {
			select {
			case <-keepalive:
				last = time.Now()
			case <-time.After(30 * time.Second):
				glog.Warningln("timeout receive data from websocket server! last_keepalive:", last.Format("2006-01-02 15:04:05"))
				reconnect()
				last = time.Now()
			}
		}
	}()
	for {
		if w.conn == nil {
			time.Sleep(300 * time.Millisecond)
			continue
		}
		_, message, err := w.conn.ReadMessage()
		if err != nil {
			glog.Errorln("conn.ReadMessage failed!", err)
			time.Sleep(300 * time.Millisecond)
			continue
		}
		message = decompress(message)
		var body struct {
			Table string `json:"table"`
		}
		json.Unmarshal(message, &body)
		//glog.Infoln("message:", string(message))
		if ch, ok := w.tbms[body.Table]; ok {
			select {
			case ch <- message:
			case <-time.After(time.Second):
				for i := 0; i < 20; i++ {
					<-ch
				}
			}

			keepalive <- 1
		}
	}
}

func (w *swapWebsocket) connect() (err error) {
	// 访问时需要科学上网
	//
	//连接说明：
	//
	//所有返回数据都进行了压缩，需要用户将接收到的数据进行解压。
	dialer := &websocket.Dialer{
		Proxy:            nil,
		HandshakeTimeout: 45 * time.Second,
	}
	w.conn, _, err = dialer.Dial("wss://real.okex.com:8443/ws/v3", nil)
	if err == nil {
		w.conn.EnableWriteCompression(false)
		w.conn.SetPingHandler(func(appData string) error {
			err := w.conn.WriteControl(websocket.PongMessage, []byte("ping"), time.Now().Add(time.Second))
			if err == websocket.ErrCloseSent {
				return nil
			} else if e, ok := err.(net.Error); ok && e.Temporary() {
				return nil
			}
			return err
		})
		w.conn.SetPongHandler(func(appData string) error {
			if appData == "pong" {
				return nil
			}
			return fmt.Errorf("want pong,  message:%s", appData)
		})
	}
	return
}
