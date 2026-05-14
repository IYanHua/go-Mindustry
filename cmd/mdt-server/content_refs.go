package main

import (
	"github.com/IYanHua/mdt-server/internal/protocol"
)

type unitTypeRef struct {
	id   int16
	name string
}

func (u unitTypeRef) ContentType() protocol.ContentType { return protocol.ContentUnit }
func (u unitTypeRef) ID() int16                         { return u.id }
func (u unitTypeRef) Name() string                      { return u.name }

type bulletTypeRef struct {
	id   int16
	name string
}

func (b bulletTypeRef) ContentType() protocol.ContentType { return protocol.ContentBullet }
func (b bulletTypeRef) ID() int16                         { return b.id }
func (b bulletTypeRef) Name() string                      { return b.name }

type itemRef struct {
	id int16
}

func (i itemRef) ContentType() protocol.ContentType { return protocol.ContentItem }
func (i itemRef) ID() int16                         { return i.id }

type blockRef struct {
	id   int16
	name string
}

func (b blockRef) ContentType() protocol.ContentType { return protocol.ContentBlock }
func (b blockRef) ID() int16                         { return b.id }
func (b blockRef) Name() string                      { return b.name }

