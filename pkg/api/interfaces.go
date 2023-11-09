package api

import "context"

type Reloader interface {
	Reload(ctx context.Context, apis ParamsAPI) error
}

type Normalizer interface {
	Normalize()
}
