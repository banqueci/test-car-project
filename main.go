package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"net/http"
	"time"
)

const (
	sk_a = "d8e98247de9d8ca2740bffa4c7c881a9393cb175bea8d14d61420a3a888f2c1e"
	sk_b = "20a5f842b33b05ce853be233e15fdca6822a12548375289d513f3063c413bcb0"
	npub_a = "npub1e7r6zt0dqgh38snlghvk2pk6m8qy5ra5xwxvr85jmv5cja5tds7scxsvy6"
	npub_b = "npub1y5whtkawpts9vnt7s38gcaf0sq9adv0f9akt97ewkfslm0gr9hnqxvnw0t"
)

var wsupgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(_ *http.Request) bool {
		return true
	},
}

type Client struct {
	conn	*websocket.Conn
	chn 	chan string
}
var Cli *Client

func main()  {
	//1、启动连接

	var relays []*nostr.Relay
	urls := []string{"ws://110.41.16.146:2700", "ws://182.61.59.53:2700"}
	for _,url := range urls{
		relay, e := nostr.RelayConnect(context.Background(), url)
		if e != nil {
			fmt.Println(e)
			continue
		}
		relays = append(relays, relay)
	}

	//2、初始化ws连接
	r := gin.Default()
	r.LoadHTMLFiles("index.html")


	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", nil)
	})

	r.GET("/ws", func(c *gin.Context) {
		//监听客户端长连接消息，并订阅relayer
		//client := NewClient(c.Writer, c.Request)
		Cli = NewClient(c.Writer, c.Request)
		Cli.wshandler(relays)
		Cli.sendMsg()
	})

	r.Run("127.0.0.1:12312")

	////3、监听长连接消息，接收到时发布
	//for {
	//	// 接收客户端message
	//	_, message, err := WsConn.ReadMessage()
	//	if err != nil {
	//		log.Println("read:", err)
	//		break
	//	}
	//	log.Printf("recv: %s", message)
	//	//向relayer发布message
	//	for _,value := range relays{
	//		fmt.Println("60:publish message", string(message))
	//		Publish(value, context.Background(), string(message))
	//	}
	//}
}

// Publish 向指定relay连接发送内容为content的事件
func Publish(relay *nostr.Relay, ctx context.Context, content string){
	pub,_ := nostr.GetPublicKey(sk_a)
	ev := nostr.Event{
		PubKey:    pub,
		CreatedAt: time.Now(),
		Kind:      nostr.KindTextNote,
		Tags:      nil,
		Content:   content,
	}
	err := ev.Sign(sk_a)
	if err != nil {
		return
	}

	fmt.Println("publish msg to ", relay.URL, relay.Publish(ctx, ev))
}

func (client *Client) Subscribe(relay *nostr.Relay, cont context.Context){
	var filters nostr.Filters
	if _, v, err := nip19.Decode(npub_b); err == nil {
		pub := v.(string)
		filters = []nostr.Filter{{
			Kinds:   []int{nostr.KindTextNote},
			Authors: []string{pub},
			Limit:   1,
		}}
	} else {
		panic(err)
	}

	ctx, _ := context.WithCancel(cont)
	sub := relay.Subscribe(ctx, filters)
	go func() {
		<-sub.EndOfStoredEvents
		// handle end of stored events (EOSE, see NIP-15)
	}()

	for ev := range sub.Events {
		// 处理监听到的事件（channel将持续开启，直到ctx被cancelled）
		fmt.Println("handle relayer msg:", ev.ID, "=======", ev.PubKey, "=======", ev.Content)

		//将消息通过ws发送给客户端
		fmt.Println("send relayer to client:", ev.Content)
		client.conn.WriteMessage(1, []byte(ev.Content))
	}
}

func (client *Client) wshandler(relays []*nostr.Relay) {
	//先开启订阅relayer
	for _,relayUrl := range relays{
		go client.Subscribe(relayUrl, context.Background())
	}

	//再监听长连接消息，监听到则发送到relayer
	for {
		t, msg, err := client.conn.ReadMessage()
		if err != nil {
			break
		}
		fmt.Println("receive ws msg:", t, string(msg))
		for _,value := range relays{
			fmt.Println("send msg to relayers:", t, string(msg))
			Publish(value, context.Background(), string(msg))
		}
	}
}

func (client *Client) sendMsg() {
	for {
		select {
		case msg := <- client.chn:
			fmt.Println("send ws msg to client:", 1, msg)
			err := client.conn.WriteMessage(1, []byte(msg))
			if err != nil {
				return
			}
		}
	}
}

func NewClient (w http.ResponseWriter, r *http.Request) *Client{
	conn, err := wsupgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("Failed to set websocket upgrade: %+v\n", err)
		return nil
	}
	client := &Client{
		conn: conn,
		chn: make(chan string),
	}
	return client
}
