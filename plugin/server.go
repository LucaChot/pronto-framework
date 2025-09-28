package plugin

import (
	"context"
	"io"
	"log"
	"net"

	pb "github.com/LucaChot/pronto-framework/message"
	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)



func (ps *prontoState) startPlacementServer(ctx context.Context, logger logr.Logger) {
    lis, err := net.Listen("tcp", ":50051")
	if err != nil {
        log.Fatalf("(grpc) failed to start server %s", err)
	}

	s := grpc.NewServer()
    pb.RegisterSignalServiceServer(s, ps)

    log.Printf("(grpc) started server on %s", lis.Addr().String())

	go func() {
		if err := s.Serve(lis); err != nil {
            log.Fatalf("(grpc) failed to serve clients %s", err)
		}
	}()

	go func() {
        <-ctx.Done()
        log.Print("(grpc) shutting down gRPC server")
        s.GracefulStop()
        log.Print("(grpc) server shutdown complete")
	}()

    if logger.V(4).Enabled() {
        logger.Info("Successfully started server")
    }

}

func (ps *prontoState) StreamSignals(stream pb.SignalService_StreamSignalsServer) error {
    var m pb.Signal
    var node string
    for {
        err := stream.RecvMsg(&m)
        if err != nil {
            if node != "" {
                ps.UpdateHostInfo(node, WithCapacity(m.Capacity),
                    WithSignal(m.Signal),
                    WithOverprovision(m.Overprovision))
            }
            if err == io.EOF {
                log.Printf("Client %s disconnected gracefully.", node)
                return nil
            }
            log.Printf("Error receiving stream from client %s: %v", node, err)
			return status.Errorf(codes.Internal, "error receiving stream: %v", err)
        }

        if node == "" {
            node = m.GetNode()
        }
        ps.UpdateHostInfo(node, WithCapacity(m.Capacity),
            WithSignal(m.Signal),
            WithOverprovision(m.Overprovision))
    }
}

