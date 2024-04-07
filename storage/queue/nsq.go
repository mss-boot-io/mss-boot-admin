/*
 * @Author: lwnmengjing
 * @Date: 2021/5/30 7:30 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/5/30 7:30 下午
 */

package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nsqio/go-nsq"

	"github.com/mss-boot-io/mss-boot-admin/storage"
)

// NewNSQ nsq模式 只能监听一个channel
func NewNSQ(cfg *nsq.Config, lookup, adminAddr string, addresses ...string) (*NSQ, error) {
	n := &NSQ{
		lookupAddr: lookup,
		addresses:  addresses,
		adminAddr:  adminAddr,
		cfg:        cfg,
	}
	//通过adminaddr获取节点信息
	n.queryNSQAdmin()

	//var err error
	//if len(addresses) > 1 {
	//	n.producer, err = n.newProducer()
	//}
	return n, nil
}

type NSQ struct {
	addresses  []string
	lookupAddr string
	adminAddr  string
	cfg        *nsq.Config
	producer   *nsq.Producer
	consumer   *nsq.Consumer
	mux        sync.Mutex
}

// String 字符串类型
func (*NSQ) String() string {
	return "nsq"
}

// switchAddress ⚠️生产环境至少配置三个节点
func (e *NSQ) switchAddress() {
	if len(e.addresses) > 1 {
		e.addresses[0], e.addresses[len(e.addresses)-1] =
			e.addresses[1],
			e.addresses[0]
	}
}

func (e *NSQ) newProducer() (*nsq.Producer, error) {
	e.mux.Lock()
	defer e.mux.Unlock()
	defer e.switchAddress()
	if e.cfg == nil {
		e.cfg = nsq.NewConfig()
	}
	return nsq.NewProducer(e.addresses[0], e.cfg)
}

func (e *NSQ) newConsumer(topic, channel string, h nsq.Handler) (err error) {
	if e.cfg == nil {
		e.cfg = nsq.NewConfig()
	}
	if e.consumer == nil {
		e.consumer, err = nsq.NewConsumer(topic, channel, e.cfg)
		if err != nil {
			return err
		}
	}
	e.consumer.AddHandler(h)
	if e.lookupAddr != "" {
		err = e.consumer.ConnectToNSQLookupd(e.lookupAddr)
		return
	}
	err = e.consumer.ConnectToNSQDs(e.addresses)
	return err
}

// Append 消息入生产者
func (e *NSQ) Append(message storage.Messager) error {
	rb, err := json.Marshal(message.GetValues())
	if err != nil {
		return err
	}
	if e.producer == nil {
		e.producer, err = e.newProducer()
		if err != nil {
			return err
		}
	}
	var count int
RETRY:
	{
		err = e.producer.Publish(message.GetStream(), rb)
		if err != nil {
			count++
			if count >= len(e.addresses) {
				return err
			}
			err = nil
			goto RETRY
		}
	}
	return err
}

// Register 监听消费者
func (e *NSQ) Register(name, channel string, f storage.ConsumerFunc) {
	h := &nsqConsumerHandler{f}
	err := e.newConsumer(name, channel, h)
	if err != nil {
		//目前不支持动态注册
		panic(err)
	}
}

func (e *NSQ) ping() {
	for {
		err := e.producer.Ping()
		if err != nil {
			e.switchAddress()
			e.producer, _ = e.newProducer()
		}
		time.Sleep(5 * time.Second)
	}
}

func (e *NSQ) Run(context.Context) {
	e.ping()
}

func (e *NSQ) Shutdown() {
	if e.producer != nil {
		e.producer.Stop()
	}
	if e.consumer != nil {
		e.consumer.Stop()
	}
}

func (e *NSQ) queryNSQAdmin() {
	if e.adminAddr == "" {
		return
	}
	endpoint := e.adminAddr
	if strings.Index(endpoint, "http") < 0 {
		endpoint = fmt.Sprintf("http://%s", endpoint)
	}

	var data NodesResp
	client := &http.Client{}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/nodes", endpoint), nil)
	if err != nil {
		slog.Error("error creating HTTP request to nsq admin", slog.Any("err", err))
		return
	}
	if e.cfg.AuthSecret != "" && e.cfg.LookupdAuthorization {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.cfg.AuthSecret))
	}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("error querying nsq admin", slog.Any("err", err))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		slog.Error("error querying nsq admin", slog.Any("status_code", resp.StatusCode))
		return
	}
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		slog.Error("error decoding nsq admin response", slog.Any("err", err))
		return
	}

	for i := range data.Nodes {
		broadcastAddress := data.Nodes[i].BroadcastAddress
		port := data.Nodes[i].TCPPort
		joined := net.JoinHostPort(broadcastAddress, strconv.Itoa(port))
		var exist bool
		for j := range e.addresses {
			if e.addresses[j] == joined {
				exist = true
				break
			}
		}
		if !exist {
			e.addresses = append(e.addresses, joined)
		}
	}
}

type NodesResp struct {
	Nodes   []*peerInfo `json:"nodes"`
	Message string      `json:"message"`
}

type peerInfo struct {
	RemoteAddress    string `json:"remote_address"`
	Hostname         string `json:"hostname"`
	BroadcastAddress string `json:"broadcast_address"`
	TCPPort          int    `json:"tcp_port"`
	HTTPPort         int    `json:"http_port"`
	Version          string `json:"version"`
}

type nsqConsumerHandler struct {
	f storage.ConsumerFunc
}

func (e nsqConsumerHandler) HandleMessage(message *nsq.Message) error {
	m := new(Message)
	data := make(map[string]interface{})
	err := json.Unmarshal(message.Body, &data)
	if err != nil {
		return err
	}
	m.SetValues(data)
	return e.f(m)
}
