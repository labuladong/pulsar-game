package main

import (
	"context"
	"encoding/json"
	"github.com/apache/pulsar-client-go/pulsar"
	log "github.com/sirupsen/logrus"
	"math"
	"time"
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
	List   []int  `json:"list"`
}

type pulsarClient struct {
	topicName, subscriptionName string
	client                      pulsar.Client
	producer                    pulsar.Producer
	consumer                    pulsar.Consumer
	consumeCh                   chan pulsar.ConsumerMessage
	// exclude type
	exclusiveObstacleConsumer pulsar.Consumer
	// to read the latest obstacle graph
	obstacleReader pulsar.Reader
	// subscribe the obstacle topic,
	closeCh chan struct{}
}

func (c *pulsarClient) Close() {
	c.producer.Close()
	c.consumer.Close()
	c.client.Close()
	c.closeCh <- struct{}{}
	close(c.closeCh)
	close(c.consumeCh)
}

func newPulsarClient(topicName, subscriptionName, keyPath string) *pulsarClient {
	oauthConfig := map[string]string{
		"type":       "client_credentials",
		"issuerUrl":  "https://auth.streamnative.cloud/",
		"audience":   "urn:sn:pulsar:o-7udlj:free",
		"privateKey": keyPath,
		"clientId":   "fdl_test",
	}
	oauth := pulsar.NewAuthenticationOAuth2(oauthConfig)
	client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL:            pulsarUrl,
		Authentication: oauth,
	})
	if err != nil {
		log.Fatal(err)
	}

	// player event topicName
	producer, err := client.CreateProducer(pulsar.ProducerOptions{
		Topic:           topicName,
		DisableBatching: true,
	})
	if err != nil {
		log.Fatal(err)
	}
	consumeCh := make(chan pulsar.ConsumerMessage)
	consumer, err := client.Subscribe(pulsar.ConsumerOptions{
		Topic:            topicName,
		SubscriptionName: subscriptionName,
		Type:             pulsar.Exclusive,
		MessageChannel:   consumeCh,
	})
	if err != nil {
		log.Fatal("this player has logged in")
	}
	// only handle new event
	err = consumer.SeekByTime(time.Now())

	if err != nil {
		log.Fatal(err)
	}

	return &pulsarClient{
		topicName:        topicName,
		subscriptionName: subscriptionName,
		client:           client,
		producer:         producer,
		consumer:         consumer,
		consumeCh:        consumeCh,
		closeCh:          make(chan struct{}),
	}
}

// try grab exclusive consumer, if success, send new random graph
func (c *pulsarClient) tryUpdateObstacles() {
	obstacleTopicName := c.topicName + "-obstacle"
	// every minute update random obstacle
	if c.exclusiveObstacleConsumer == nil {
		obstacleSubscriptionName := obstacleTopicName + "-sub"
		obstacleConsumerCh := make(chan pulsar.ConsumerMessage)
		obstacleConsumer, err := c.client.Subscribe(pulsar.ConsumerOptions{
			Topic: obstacleTopicName,
			// all player clients should have same subscription name
			// then fail-over type can work
			SubscriptionName: obstacleSubscriptionName,
			// only one consumer can subscribe obstacle topic
			Type:                        pulsar.Exclusive,
			MessageChannel:              obstacleConsumerCh,
			SubscriptionInitialPosition: pulsar.SubscriptionPositionLatest,
		})
		if err != nil {
			// subscription already has other consumers
			return
		}
		c.exclusiveObstacleConsumer = obstacleConsumer
	}

	// now, this player is the first consumer, update the map
	// obstacle topic producer
	producer, err := c.client.CreateProducer(pulsar.ProducerOptions{
		Topic:           obstacleTopicName,
		DisableBatching: true,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer producer.Close()

	total := xGridCountInScreen * yGridCountInScreen

	InitObstacleMsg := &EventMessage{
		Type: InitObstacleEventType,
		List: sample(total, total/obstacleRatio),
	}
	bytes, err := json.Marshal(InitObstacleMsg)
	if err != nil {
		log.Fatal(err)
	}
	_, err = producer.Send(context.Background(), &pulsar.ProducerMessage{Payload: bytes})
}

func (c *pulsarClient) readLatestEvent(topicName string) Event {
	reader, err := c.client.CreateReader(pulsar.ReaderOptions{
		Topic: topicName,
		// get the latest message
		StartMessageID:          pulsar.LatestMessageID(),
		StartMessageIDInclusive: true,
	})
	if err != nil {
		log.Error(err)
	}
	defer reader.Close()

	if reader.HasNext() {
		msg, err := reader.Next(context.Background())
		if err != nil {
			log.Error(err)
		}
		actionMsg := EventMessage{}

		err = json.Unmarshal(msg.Payload(), &actionMsg)
		return convertMsgToEvent(&actionMsg)
	}
	return nil
}

// start to receive message from pulsar, forwarding to receiveCh
func (c *pulsarClient) start(in chan Event) chan Event {
	// All players' action can be received from this channel
	outCh := make(chan Event)
	go func() {
		for {
			select {
			// receive message from pulsar, forwarding to outCh
			case cm := <-c.consumeCh:
				msg := cm.Message
				if msg == nil {
					log.Warning("receive a nil message")
					break
				}
				actionMsg := EventMessage{}
				err := json.Unmarshal(msg.Payload(), &actionMsg)
				if err != nil {
					log.Fatal(err)
				}
				l := math.Min(float64(len(msg.Payload())), 100)
				log.Info("receive message from pulsar:\n", string(msg.Payload())[:int(l)])
				cm.Ack(msg)
				outCh <- convertMsgToEvent(&actionMsg)

			// need to send message to pulsar
			case action := <-in:
				if action == nil {
					log.Warning("send a nil message")
					break
				}
				actionMsg := convertEventToMsg(action)
				bytes, err := json.Marshal(actionMsg)
				if err != nil {
					log.Fatal(err)
				}
				_, err = c.producer.Send(context.Background(), &pulsar.ProducerMessage{Payload: bytes})
				if err != nil {
					return
				}
				//log.Info("send message to pulsar:\n", string(bytes))

			case <-c.closeCh:
				goto stop
			}
		stop:
		}
	}()

	// handle obstacle topic
	go func() {
		// 1. try to init random map
		c.tryUpdateObstacles()

		// 2. read the latest random map
		obstacleTopicName := c.topicName + "-obstacle"
		event := c.readLatestEvent(obstacleTopicName)
		if event != nil {
			outCh <- event
		}
		// 3. create consumer listener
		obstacleConsumerCh := make(chan pulsar.ConsumerMessage)
		consumer, err := c.client.Subscribe(pulsar.ConsumerOptions{
			Topic:            obstacleTopicName,
			SubscriptionName: c.subscriptionName,
			Type:             pulsar.Exclusive,
			MessageChannel:   obstacleConsumerCh,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer consumer.Close()
		err = consumer.SeekByTime(time.Now())
		if err != nil {
			log.Fatal(err)
		}

		for {
			select {
			case <-time.Tick(time.Second * updateObstacleTime):
				// every minute update random obstacle
				c.tryUpdateObstacles()
			case cm := <-obstacleConsumerCh:
				msg := cm.Message
				if err != nil {
					log.Error(err)
				}
				actionMsg := EventMessage{}

				err = json.Unmarshal(msg.Payload(), &actionMsg)
				outCh <- convertMsgToEvent(&actionMsg)
			}
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
			Name:   t.name,
			Avatar: t.avatar,
			X:      t.pos.X,
			Y:      t.pos.Y,
			Alive:  t.alive,
		}
	case *UserDeadEvent:
		msg = &EventMessage{
			Type:   UserDeadEventType,
			Name:   t.name,
			Avatar: t.avatar,
			X:      t.pos.X,
			Y:      t.pos.Y,
			Alive:  false,
		}
	case *UserReviveEvent:
		msg = &EventMessage{
			Type:   UserReviveEventType,
			Name:   t.name,
			Avatar: t.avatar,
			X:      t.pos.X,
			Y:      t.pos.Y,
			Alive:  true,
		}
	case *SetBombEvent:
		msg = &EventMessage{
			Type: SetBombEventType,
			Name: t.bombName,
			X:    t.pos.X,
			Y:    t.pos.Y,
		}
	case *BombMoveEvent:
		msg = &EventMessage{
			Type: MoveBombEventType,
			Name: t.bombName,
			X:    t.pos.X,
			Y:    t.pos.Y,
		}
	case *ExplodeEvent:
		msg = &EventMessage{
			Type: ExplodeEventType,
			Name: t.bombName,
			X:    t.pos.X,
			Y:    t.pos.Y,
		}
	case *UndoExplodeEvent:
		msg = &EventMessage{
			Type: UndoExplodeEventType,
			X:    t.pos.X,
			Y:    t.pos.Y,
		}
	case *InitObstacleEvent:
		msg = &EventMessage{
			Type: InitObstacleEventType,
			List: t.Obstacles,
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
		return &SetBombEvent{
			bombName: msg.Name,
			pos:      info.pos,
		}
	case MoveBombEventType:
		return &BombMoveEvent{
			bombName: msg.Name,
			pos:      info.pos,
		}
	case UserMoveEventType:
		return &UserMoveEvent{
			playerInfo: info,
		}
	case UserDeadEventType:
		return &UserDeadEvent{
			playerInfo: info,
		}
	case UserReviveEventType:
		return &UserReviveEvent{
			playerInfo: info,
		}
	case ExplodeEventType:
		return &ExplodeEvent{
			bombName: msg.Name,
			pos:      info.pos,
		}
	case UndoExplodeEventType:
		return &UndoExplodeEvent{
			pos: info.pos,
		}
	case InitObstacleEventType:
		return &InitObstacleEvent{
			Obstacles: msg.List,
		}
	}
	return nil
}
