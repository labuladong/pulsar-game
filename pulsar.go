package main

import (
	"context"
	"encoding/json"
	"github.com/apache/pulsar-client-go/pulsar"
	log "github.com/sirupsen/logrus"
)

// EventMessage is the data in Pulsar
type EventMessage struct {
	// Event type
	Type   string `json:"type"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Alive  bool   `json:"alive"`
}

type pulsarClient struct {
	topicName string
	client    pulsar.Client
	producer  pulsar.Producer
	consumer  pulsar.Consumer
	consumeCh chan pulsar.ConsumerMessage
	closeCh   chan struct{}
}

func (c *pulsarClient) Close() {
	c.producer.Close()
	c.consumer.Close()
	c.client.Close()
	c.closeCh <- struct{}{}
	close(c.closeCh)
	close(c.consumeCh)
}

func newPulsarClient(topic, subscriptionName, keyPath string) *pulsarClient {
	oauthConfig := map[string]string{
		"type":       "client_credentials",
		"issuerUrl":  "https://auth.streamnative.cloud/",
		"audience":   "urn:sn:pulsar:sndev:kj-game",
		"privateKey": keyPath,
		"clientId":   "fdl_test",
	}
	oauth := pulsar.NewAuthenticationOAuth2(oauthConfig)
	client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL:            "pulsar+ssl://kj-game.sndev.snio.cloud:6651",
		Authentication: oauth,
	})
	if err != nil {
		log.Fatal(err)
	}

	producer, err := client.CreateProducer(pulsar.ProducerOptions{
		Topic:           topic,
		DisableBatching: true,
	})
	if err != nil {
		log.Fatal(err)
	}

	consumeCh := make(chan pulsar.ConsumerMessage)
	consumer, err := client.Subscribe(pulsar.ConsumerOptions{
		Topic:            topic,
		SubscriptionName: subscriptionName,
		Type:             pulsar.Shared,
		MessageChannel:   consumeCh,
	})
	if err != nil {
		log.Fatal(err)
	}

	return &pulsarClient{
		topicName: topic,
		producer:  producer,
		consumer:  consumer,
		consumeCh: consumeCh,
		closeCh:   make(chan struct{}),
	}
}

// start to receive message from pulsar, forwarding to receiveCh
func (c *pulsarClient) start(in chan Event) chan Event {
	// Other players' action can be received from this channel
	outCh := make(chan Event)
	go func() {
		for {
			select {
			// receive message from pulsar, forwarding to outCh
			case cm := <-c.consumeCh:
				msg := cm.Message
				actionMsg := EventMessage{}
				err := json.Unmarshal(msg.Payload(), &actionMsg)
				if err != nil {
					log.Fatal(err)
				}
				log.Warning("receive message from pulsar:\n", string(msg.Payload()))
				outCh <- convertMsgToEvent(&actionMsg)

			// need to send message to pulsar
			case action := <-in:
				actionMsg := convertEventToMsg(action)
				bytes, err := json.Marshal(actionMsg)
				if err != nil {
					log.Fatal(err)
				}
				_, err = c.producer.Send(context.Background(), &pulsar.ProducerMessage{Payload: bytes})
				if err != nil {
					return
				}
				log.Warning("send message to pulsar:\n", string(bytes))

			case <-c.closeCh:
				goto stop
			}
		stop:
		}
	}()
	return outCh
}

func convertEventToMsg(action Event) *EventMessage {
	var msg *EventMessage
	switch t := action.(type) {
	case *UserMoveEvent:
		msg = &EventMessage{
			Type:   UserMoveEventType,
			Name:   t.name,
			Avatar: t.avatar,
			X:      t.pos.X,
			Y:      t.pos.Y,
			Alive:  t.alive,
		}
	case *UserJoinEvent:
		msg = &EventMessage{
			Type:   UserJoinEventType,
			Name:   t.playerInfo.name,
			Avatar: t.playerInfo.avatar,
			X:      t.pos.X,
			Y:      t.pos.Y,
			Alive:  t.alive,
		}
	case *SetBoomEvent:
		msg = &EventMessage{
			Type:   SetBombEventType,
			Name:   t.playerInfo.name,
			Avatar: t.playerInfo.avatar,
			X:      t.pos.X,
			Y:      t.pos.Y,
			Alive:  t.alive,
		}
	case *ExplodeEvent:
		msg = &EventMessage{
			Type: ExplodeEventType,
			Name: t.name,
			X:    t.pos.X,
			Y:    t.pos.Y,
		}
	case *UndoExplodeEvent:
		msg = &EventMessage{
			Type: UndoExplodeEventType,
			X:    t.pos.X,
			Y:    t.pos.Y,
		}
	}
	return msg
}

func convertMsgToEvent(msg *EventMessage) Event {
	info := &playerInfo{
		name:   msg.Name,
		avatar: msg.Avatar,
		pos: Position{
			X: msg.X,
			Y: msg.Y,
		},
		alive: msg.Alive,
	}
	switch msg.Type {
	case UserJoinEventType:
		return &UserJoinEvent{
			playerInfo: info,
		}
	case SetBombEventType:
		return &SetBoomEvent{
			playerInfo: info,
		}
	case UserMoveEventType:
		return &UserMoveEvent{
			playerInfo: info,
		}
	case ExplodeEventType:
		return &ExplodeEvent{
			name: info.name,
			pos:  info.pos,
		}
	case UndoExplodeEventType:
		return &UndoExplodeEvent{
			pos: info.pos,
		}
	}
	return nil
}
