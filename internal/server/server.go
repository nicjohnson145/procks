package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	"connectrpc.com/connect"
	"github.com/bufbuild/protovalidate-go"
	pbv1 "github.com/nicjohnson145/procks/gen/procks/v1"
	connectv1 "github.com/nicjohnson145/procks/gen/procks/v1/procksv1connect"
	"github.com/oklog/ulid/v2"
	"github.com/rs/zerolog"
)

type ServerConfig struct {
	Logger zerolog.Logger
	Url    string
}

func NewServer(conf ServerConfig) *Server {
	return &Server{
		log:          conf.Logger,
		url:          conf.Url,
		openHandlers: make(map[string]map[string]*connect.ServerStream[pbv1.ConnectResponse]),
	}
}

type Server struct {
	connectv1.UnimplementedProcksServiceHandler

	log zerolog.Logger
	url string

	mu           sync.RWMutex
	openHandlers map[string]map[string]*connect.ServerStream[pbv1.ConnectResponse]
}

func (s *Server) logAndHandleError(err error, msg string) error {
	str := "an error occurred"
	if msg != "" {
		str = msg
	}

	s.log.Err(err).Msg(str)

	var validationError *protovalidate.ValidationError
	isProtoValidation := errors.As(err, &validationError)

	switch true {
	case isProtoValidation:
		return connect.NewError(connect.CodeInvalidArgument, err)
	default:
		return err
	}
}

func (s *Server) Connect(ctx context.Context, req *connect.Request[pbv1.ConnectRequest], stream *connect.ServerStream[pbv1.ConnectResponse]) error {
	if err := s.validateConnect(req); err != nil {
		return s.logAndHandleError(err, "error validating")
	}

	// The "url" ID, can have multiple clients recieving messages
	var streamID string
	if req.Msg.Id == nil {
		streamID = ulid.Make().String()
	} else {
		streamID = *req.Msg.Id
	}

	// request ID, only one of these, ensures that stream handlers get cleaned up
	requestID := ulid.Make().String()

	s.log.Info().Msgf("recieved connect request for stream ID %v", streamID)

	s.log.Debug().Msg("sending connection established message to new client")
	establishedResp := &pbv1.ConnectResponse{
		Event: &pbv1.Event{
			Message: &pbv1.Event_ConnectionEstablished{
				ConnectionEstablished: &pbv1.Event_ConnectionEstablishedEvent{
					Id:  streamID,
					Url: fmt.Sprintf("%v/%v", s.url, streamID),
				},
			},
		},
	}
	if err := stream.Send(establishedResp); err != nil {
		return s.logAndHandleError(err, "error sending initial establish response")
	}

	s.registerHandler(streamID, requestID, stream)

	s.log.Debug().Msg("sitting in channel wait until stream closes")
	<-ctx.Done()

	s.unregisterHandler(streamID, requestID)

	return nil
}

func (s *Server) validateConnect(req *connect.Request[pbv1.ConnectRequest]) error {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return err
	}
	return nil
}

func (s *Server) registerHandler(streamID string, requestID string, stream *connect.ServerStream[pbv1.ConnectResponse]) {
	s.log.Debug().Msg("registering stream handler")

	s.mu.Lock()
	existingHandlers, ok := s.openHandlers[streamID]
	if !ok {
		existingHandlers = make(map[string]*connect.ServerStream[pbv1.ConnectResponse])
	}
	existingHandlers[requestID] = stream
	s.openHandlers[streamID] = existingHandlers

	s.mu.Unlock()
}

func (s *Server) unregisterHandler(streamID string, requestID string) {
	s.log.Debug().Msg("de-registering stream handler")

	s.mu.Lock()
	existingHandlers, ok := s.openHandlers[streamID]
	if !ok {
		existingHandlers = make(map[string]*connect.ServerStream[pbv1.ConnectResponse])
	}
	delete(existingHandlers, requestID)
	if len(existingHandlers) == 0 {
		delete(s.openHandlers, streamID)
	} else {
		s.openHandlers[streamID] = existingHandlers
	}

	s.mu.Unlock()
}

func (s *Server) RequestHandler() (string, http.HandlerFunc) {
	return "/{streamID}/{path...}", func(w http.ResponseWriter, r *http.Request) {
		s.log.
			Info().
			Str("streamID", r.PathValue("streamID")).
			Str("path", r.PathValue("path")).
			Str("method", r.Method).
			Msg("standard request recieved")

		reqBody, err := io.ReadAll(r.Body)
		if err != nil {
			s.logAndHandleError(err, "error reading incoming request body")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Yeah i know you can have multiple values, but like....its a dev server. its fine
		outHeaders := map[string]string{}
		for header, valList := range r.Header {
			val := ""
			if len(valList) > 0 {
				val = valList[0]
			}
			outHeaders[header] = val
		}

		payload := &pbv1.ConnectResponse{
			Event: &pbv1.Event{
				Message: &pbv1.Event_RequestRecieved{
					RequestRecieved: &pbv1.Event_RequestRecievedEvent{
						Verb:    r.Method,
						Path:    r.PathValue("path"),
						Body:    reqBody,
						Headers: outHeaders,
					},
				},
			},
		}

		s.mu.RLock()
		defer s.mu.RUnlock()
		handlers, ok := s.openHandlers[r.PathValue("streamID")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		for _, stream := range handlers {
			if err := stream.Send(payload); err != nil {
				s.log.Err(err).Msg("error sending payload")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		w.WriteHeader(http.StatusOK)
	}
}
