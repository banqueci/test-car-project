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

var Ctx *gin.Context

func main()  {
	//1、启动时连接并订阅relayer
	var relays []*nostr.Relay
	urls := []string{"ws://110.41.16.146:2700", "ws://182.61.59.53:2700"}

	for _,url := range urls{
		relay, e := nostr.RelayConnect(context.Background(), url)
		if e != nil {
			fmt.Println(e)
			continue
		}
		go Subscribe(relay, context.Background())
		relays = append(relays, relay)
	}

	//2、初始化ws连接
	r := gin.Default()
	r.LoadHTMLFiles("index.html")

	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", nil)
		//Ctx = c
	})

	r.GET("/ws", func(c *gin.Context) {
		wshandler(c.Writer, c.Request, relays)
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
func Publish(r *nostr.Relay, ctx context.Context, content string){
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

	fmt.Println("publish msg to ", r.URL, r.Publish(ctx, ev))
}

func Subscribe(relay *nostr.Relay, c context.Context){
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

	ctx, _ := context.WithCancel(c)
	sub := relay.Subscribe(ctx, filters)
	go func() {
		<-sub.EndOfStoredEvents
		// handle end of stored events (EOSE, see NIP-15)
	}()

	for ev := range sub.Events {
		// 处理监听到的事件（channel将持续开启，直到ctx被cancelled）
		fmt.Println(ev.ID, "=======", ev.PubKey, "=======", ev.Content)

		//通过ws向客户端发送message
		//err := conn.WriteMessage(1, []byte(ev.Content))
		//if err != nil {
		//	log.Println("write:", err)
		//	break
		//}
		//sendMsg(Ctx.Writer, Ctx.Request, ev.Content)
	}
}

func wshandler(w http.ResponseWriter, r *http.Request, relays []*nostr.Relay) {
	conn, err := wsupgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("Failed to set websocket upgrade: %+v\n", err)
		return
	}

	for {
		t, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		fmt.Println("receive ws msg:", t, string(msg))
		for _,value := range relays{
			fmt.Println("send client msg to relayers:", t, string(msg))
			Publish(value, context.Background(), string(msg))
		}

		//err = conn.WriteMessage(t, msg)
		//if err != nil {
		//	return
		//}
	}
}

func sendMsg(w http.ResponseWriter, r *http.Request, msg string) {
	conn, err := wsupgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("Failed to set websocket upgrade: %+v\n", err)
		return
	}

	for {
		//t, msg, err := conn.ReadMessage()
		//if err != nil {
		//	break
		//}
		fmt.Println("send ws msg:", 1, msg)
		err = conn.WriteMessage(1, []byte(msg))
		if err != nil {
			return
		}
	}
}
