package usecase

import "context"

// UseCase represents application use cases having input I and output O.
type UseCase[I any, O any] interface {
	Execute(ctx context.Context, in *I) (*O, error)
}
