package dea

import (
	"sync"
)

type ContainerPool struct {
	containers map[string]*Container
	mutex      sync.Mutex
}

func NewPool() *ContainerPool {
	return &ContainerPool{
		mutex:      sync.Mutex{},
		containers: make(map[string]*Container, 0),
	}
}

func (p *ContainerPool) GetAllContainers() map[string]*Container {
	return p.containers
}

func (p *ContainerPool) GetContainerByToken(token string) *Container {
	return p.containers[token]
}
