// Copyright 2013 Beego Samples authors
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package controllers

import (
	"container/list"
	"time"
    "fmt"
    "net"
    "strings"
    "encoding/json"
	"github.com/astaxie/beego"
	"github.com/gorilla/websocket"
    "github.com/garyburd/redigo/redis"
	"github.com/beego/samples/WebIM/models"
)

type Subscription struct {
	Archive []models.Event      // All the events from the archive.
	New     <-chan models.Event // New events coming in.
}

func newEvent(ep models.EventType, user, msg string) models.Event {
	return models.Event{ep, user, int(time.Now().Unix()), msg}
}

func Join(user string, ws *websocket.Conn) {
	subscribe <- Subscriber{Name: user, Conn: ws}
}

func Leave(user string) {
	unsubscribe <- user
}

type Subscriber struct {
	Name string
	Conn *websocket.Conn // Only for WebSocket users; otherwise nil.
}

var (
	// Channel for new join users.
	subscribe = make(chan Subscriber, 10)
	// Channel for exit users.
	unsubscribe = make(chan string, 10)
	// Send events here to publish them.
	publish = make(chan models.Event, 10)
	// Long polling waiting list.
	waitingList = list.New()
	subscribers = list.New()
)

// This function handles all incoming chan messages.
func chatroom() {
	for {
		select {
		case sub := <-subscribe:
			if !isUserExist(subscribers, sub.Name) {
				subscribers.PushBack(sub) // Add user to the end of list.
				// Publish a JOIN event.
				publish <- newEvent(models.EVENT_JOIN, sub.Name, "")
				beego.Info("New user c:", sub.Name, ";WebSocket:", sub.Conn != nil)
			} else {
				beego.Info("Old user c:", sub.Name, ";WebSocket:", sub.Conn != nil)
			}
		case event := <-publish:
			// Notify waiting list.
			for ch := waitingList.Back(); ch != nil; ch = ch.Prev() {
				ch.Value.(chan bool) <- true
				waitingList.Remove(ch)
			}

			broadcastWebSocket(event)
			models.NewArchive(event)

			if event.Type == models.EVENT_MESSAGE {
			beego.Info("start test")
				beego.Info("Message from c:", event.User, ";Content c:", event.Content)
			}
		case unsub := <-unsubscribe:
			for sub := subscribers.Front(); sub != nil; sub = sub.Next() {
				if sub.Value.(Subscriber).Name == unsub {
					subscribers.Remove(sub)
					// Clone connection.
					ws := sub.Value.(Subscriber).Conn
					if ws != nil {
						ws.Close()
						beego.Error("WebSocket closed:", unsub)
					}
					publish <- newEvent(models.EVENT_LEAVE, unsub, "") // Publish a LEAVE event.
					break
				}
			}
		}
	}
}

func init() {
	go chatroom()
}

func isUserExist(subscribers *list.List, user string) bool {
	for sub := subscribers.Front(); sub != nil; sub = sub.Next() {
		if sub.Value.(Subscriber).Name == user {
			return true
		}
	}
	return false
}

func setMsgQueue(uname string,content string) {
    // 从配置文件获取redis配置并连接
    host      := beego.AppConfig.String("redis_host")
    port , _  := beego.AppConfig.Int("redis_port")
    password  := beego.AppConfig.String("redis_password")

    redispool := NewRedisPool(host, port, password)
    redisconn := redispool.Get() //获取redis连接
    
    qKey := "msg_list"
    
    msgA := []string{uname,content}

    aStr , _ := json.Marshal(msgA)

    z, err := redisconn.Do("rpush", qKey, aStr)

    if err != nil {
         fmt.Println(err)
         return
    }
    beego.Info("set redis",z)
}

func NewRedisPool(host string, port int, password string) *redis.Pool {
        server := fmt.Sprintf("%s:%d", host, port)                                          
                                                                                     
        return &redis.Pool{                                                          
                MaxIdle:     3,                                                      
                IdleTimeout: 240 * time.Second,                                      
                Dial: func() (redis.Conn, error) {                                   
                        c, err := redis.Dial("tcp", server)                          
                        if err != nil {                                              
                                return nil, err                                      
                        }                                                            
                        if _, err := c.Do("AUTH", password); err != nil {            
                                c.Close()                                            
                                return nil, err                                      
                        }                                                            
                        return c, err                                                
                },                                                                   
                TestOnBorrow: func(c redis.Conn, t time.Time) error {                
                        _, err := c.Do("PING")                                       
                        return err                                                   
                },                                                                   
        }                                                                            
}

//获取本地IP                                                                         
func  GetLocalIP(ippart string) string {                              
                                                                                     
        localip := "-"                                                               
                                                                                     
        addrs, err := net.InterfaceAddrs()                                           
        if err != nil {                                                              
                beego.Error(fmt.Sprintf("get ip error:", err.Error()))                  
                return "-"                                                           
        }                                                                            
        for _, addr := range addrs {                                                 
                ip := strings.Split(addr.String(), "/")[0]                                                                                         
                if index := strings.Index(ip, ippart); index != -1 {                 
                        localip = ip                                                 
                        break                                                        
                }                                                                    
        }                                                                            
                                                                                     
        return localip                                                               
}

