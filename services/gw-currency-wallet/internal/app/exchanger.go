package app

import (
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func newExchangerConnection(address string, timeout time.Duration, options ...grpc.DialOption) (*grpc.ClientConn, error) {
	svcConfig := fmt.Sprintf(`{
  "methodConfig": [{
   "name": [{"service": "", "method": ""}],
   "timeout": "%.0fs"
  }]
 }`, timeout.Seconds())
	defaults := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(svcConfig),
	}
	return grpc.NewClient(address, append(defaults, options...)...)
}
