package client

import (
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func NewDelegatingClientFunc() manager.NewClientFunc {
	return func(cache cache.Cache, config *rest.Config, options client.Options) (client.Client, error) {
		c, err := client.New(config, options)
		if err != nil {
			return nil, err
		}
		return &client.DelegatingClient{
			Reader:       cache,
			Writer:       c,
			StatusClient: c,
		}, nil
	}
}
