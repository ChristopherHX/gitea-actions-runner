package engine

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
)

// Start start docker engine api loop
func Start(ctx context.Context) error {
	engine, err := New()
	if err != nil {
		return err
	}

	count := 0
	for {
		err := engine.Ping(ctx)
		if err == context.Canceled {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err != nil {
			log.WithError(err).
				Errorln("cannot ping the docker daemon")
			count++
			if count == 5 {
				return fmt.Errorf("retry connect to docker daemon failed: %d times", count)
			}
			time.Sleep(time.Second)
		} else {
			log.Infoln("successfully ping the docker daemon")
			break
		}
	}
	return nil
}
