package services

import (
    "context"
    "encoding/json"
    "events-service/models"
    "fmt"
    "log"

    "github.com/go-redis/redis/v8"
)

type RedisService struct {
    Client *redis.Client
    Ctx    context.Context
}

func NewRedisService(host, port string, db int) *RedisService {
    ctx := context.Background()
    
    client := redis.NewClient(&redis.Options{
        Addr:     fmt.Sprintf("%s:%s", host, port),
        Password: "",
        DB:       db,
    })

    // Test connection
    if err := client.Ping(ctx).Err(); err != nil {
        log.Printf("❌ Error conectando a Redis: %v", err)
        return nil
    }

    log.Println("✅ Conectado a Redis exitosamente")
    return &RedisService{
        Client: client,
        Ctx:    ctx,
    }
}

func (r *RedisService) PublishEvent(channel string, event models.RedisEvent) error {
    eventJSON, err := json.Marshal(event)
    if err != nil {
        return fmt.Errorf("error marshaling event: %v", err)
    }

    err = r.Client.Publish(r.Ctx, channel, string(eventJSON)).Err()
    if err != nil {
        return fmt.Errorf("error publishing event: %v", err)
    }

    log.Printf("✅ Evento publicado en %s: %s", channel, event.Type)
    return nil
}

func (r *RedisService) SubscribeToChannel(channel string, handler func(string)) {
    pubsub := r.Client.Subscribe(r.Ctx, channel)
    defer pubsub.Close()

    ch := pubsub.Channel()

    for msg := range ch {
        log.Printf("📥 Evento recibido en %s: %s", channel, msg.Payload)
        handler(msg.Payload)
    }
}

func (r *RedisService) IsConnected() bool {
    return r.Client.Ping(r.Ctx).Err() == nil
}