package cmd

import (
	"context"
	"time"

	"gitea.com/gitea/act_runner/engine"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// type Message struct {
// 	Version      int    //
// 	Type         int    // message type, 1 register 2 error
// 	RunnerUUID   string // runner uuid
// 	BuildUUID    string // build uuid
// 	ErrCode      int    // error code
// 	ErrContent   string // errors message
// 	EventName    string
// 	EventPayload string
// 	JobID        string // only run the special job, empty means run all the jobs
// }

// const (
// 	MsgTypeRegister     = iota + 1 // register
// 	MsgTypeError                   // error
// 	MsgTypeRequestBuild            // request build task
// 	MsgTypeIdle                    // no task
// 	MsgTypeBuildResult             // build result
// )

// func handleVersion1(ctx context.Context, conn *websocket.Conn, message []byte, msg *Message) error {
// 	switch msg.Type {
// 	case MsgTypeRegister:
// 		log.Info().Msgf("received registered success: %s", message)
// 		return conn.WriteJSON(&Message{
// 			Version:    1,
// 			Type:       MsgTypeRequestBuild,
// 			RunnerUUID: msg.RunnerUUID,
// 		})
// 	case MsgTypeError:
// 		log.Info().Msgf("received error msessage: %s", message)
// 		return conn.WriteJSON(&Message{
// 			Version:    1,
// 			Type:       MsgTypeRequestBuild,
// 			RunnerUUID: msg.RunnerUUID,
// 		})
// 	case MsgTypeIdle:
// 		log.Info().Msgf("received no task")
// 		return conn.WriteJSON(&Message{
// 			Version:    1,
// 			Type:       MsgTypeRequestBuild,
// 			RunnerUUID: msg.RunnerUUID,
// 		})
// 	case MsgTypeRequestBuild:
// 		switch msg.EventName {
// 		case "push":
// 			input := Input{
// 				forgeInstance:   "github.com",
// 				reuseContainers: true,
// 			}

// 			ctx, cancel := context.WithTimeout(ctx, time.Hour)
// 			defer cancel()

// 			done := make(chan error)
// 			go func(chan error) {
// 				done <- runTask(ctx, &input, "")
// 			}(done)

// 			c := time.NewTicker(time.Second)
// 			defer c.Stop()

// 			for {
// 				select {
// 				case <-ctx.Done():
// 					cancel()
// 					log.Info().Msgf("cancel task")
// 					return nil
// 				case err := <-done:
// 					if err != nil {
// 						log.Error().Msgf("runTask failed: %v", err)
// 						return conn.WriteJSON(&Message{
// 							Version:    1,
// 							Type:       MsgTypeBuildResult,
// 							RunnerUUID: msg.RunnerUUID,
// 							BuildUUID:  msg.BuildUUID,
// 							ErrCode:    1,
// 							ErrContent: err.Error(),
// 						})
// 					}
// 					log.Error().Msgf("runTask success")
// 					return conn.WriteJSON(&Message{
// 						Version:    1,
// 						Type:       MsgTypeBuildResult,
// 						RunnerUUID: msg.RunnerUUID,
// 						BuildUUID:  msg.BuildUUID,
// 					})
// 				case <-c.C:
// 				}
// 			}
// 		default:
// 			return fmt.Errorf("unknow event %s with payload %s", msg.EventName, msg.EventPayload)
// 		}
// 	default:
// 		return fmt.Errorf("received a message with an unsupported type: %#v", msg)
// 	}
// }

// // TODO: handle the message
// func handleMessage(ctx context.Context, conn *websocket.Conn, message []byte) error {
// 	var msg Message
// 	if err := json.Unmarshal(message, &msg); err != nil {
// 		return fmt.Errorf("unmarshal received message faild: %v", err)
// 	}

// 	switch msg.Version {
// 	case 1:
// 		return handleVersion1(ctx, conn, message, &msg)
// 	default:
// 		return fmt.Errorf("recevied a message with an unsupported version, consider upgrade your runner")
// 	}
// }

func runDaemon(ctx context.Context, input *Input) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		log.Infoln("Starting runner daemon")

		_ = godotenv.Load(input.envFile)
		cfg, err := fromEnviron()
		if err != nil {
			log.WithError(err).
				Fatalln("invalid configuration")
		}

		initLogging(cfg)

		opts := engine.Opts{}
		engine, err := engine.NewEnv(opts)
		if err != nil {
			log.WithError(err).
				Fatalln("cannot load the docker engine")
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
					log.WithError(err).
						Fatalln("retry count reached")
				}
				time.Sleep(time.Second)
			} else {
				log.Debugln("successfully pinged the docker daemon")
				break
			}
		}

		return nil
		// var conn *websocket.Conn
		// var err error
		// ticker := time.NewTicker(time.Second)
		// defer ticker.Stop()
		// var failedCnt int
		// for {
		// 	select {
		// 	case <-ctx.Done():
		// 		log.Info().Msgf("cancel task")
		// 		if conn != nil {
		// 			err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		// 			if err != nil {
		// 				log.Error().Msgf("write close: %v", err)
		// 			}
		// 		}
		// 		if errors.Is(ctx.Err(), context.Canceled) {
		// 			return nil
		// 		}
		// 		return ctx.Err()
		// 	case <-ticker.C:
		// 		if conn == nil {
		// 			log.Trace().Msgf("trying connect %v", "ws://localhost:3000/api/actions")
		// 			conn, _, err = websocket.DefaultDialer.DialContext(ctx, "ws://localhost:3000/api/actions", nil)
		// 			if err != nil {
		// 				log.Error().Msgf("dial: %v", err)
		// 				break
		// 			}

		// 			// register the client
		// 			msg := Message{
		// 				Version:    1,
		// 				Type:       MsgTypeRegister,
		// 				RunnerUUID: "111111",
		// 			}
		// 			bs, err := json.Marshal(&msg)
		// 			if err != nil {
		// 				log.Error().Msgf("Marshal: %v", err)
		// 				break
		// 			}

		// 			if err = conn.WriteMessage(websocket.TextMessage, bs); err != nil {
		// 				log.Error().Msgf("register failed: %v", err)
		// 				conn.Close()
		// 				conn = nil
		// 				break
		// 			}
		// 		}

		// 		const timeout = time.Second * 10

		// 		for {
		// 			select {
		// 			case <-ctx.Done():
		// 				log.Info().Msg("cancel task")
		// 				return nil
		// 			default:
		// 			}

		// 			_ = conn.SetReadDeadline(time.Now().Add(timeout))
		// 			conn.SetPongHandler(func(string) error {
		// 				return conn.SetReadDeadline(time.Now().Add(timeout))
		// 			})

		// 			_, message, err := conn.ReadMessage()
		// 			if err != nil {
		// 				if websocket.IsCloseError(err, websocket.CloseAbnormalClosure) ||
		// 					websocket.IsCloseError(err, websocket.CloseNormalClosure) {
		// 					log.Trace().Msgf("closed from remote")
		// 					conn.Close()
		// 					conn = nil
		// 				} else if !strings.Contains(err.Error(), "i/o timeout") {
		// 					log.Error().Msgf("read message failed: %#v", err)
		// 				}
		// 				failedCnt++
		// 				if failedCnt > 60 {
		// 					if conn != nil {
		// 						conn.Close()
		// 						conn = nil
		// 					}
		// 					failedCnt = 0
		// 				}
		// 				break
		// 			}

		// 			if err := handleMessage(ctx, conn, message); err != nil {
		// 				log.Error().Msgf(err.Error())
		// 			}
		// 		}
		// 	}
		// }
	}
}
