package cbornode

import (
	"math/big"
	"runtime"

	cid "github.com/ipfs/go-cid"

	encoding "github.com/ipfs/go-ipld-cbor/encoding"

	"github.com/polydawn/refmt/obj/atlas"
)

// This atlas describes the CBOR Tag (42) for IPLD links, such that refmt can marshal and unmarshal them
var cidAtlasEntry = atlas.BuildEntry(cid.Cid{}).
	UseTag(CBORTagLink).
	Transform().
	TransformMarshal(atlas.MakeMarshalTransformFunc(
		castCidToBytes,
	)).
	TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
		castBytesToCid,
	)).
	Complete()

var bigIntAtlasEntry = atlas.BuildEntry(big.Int{}).Transform().
	TransformMarshal(atlas.MakeMarshalTransformFunc(
		func(i big.Int) ([]byte, error) {
			return i.Bytes(), nil
		})).
	TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
		func(x []byte) (big.Int, error) {
			return *big.NewInt(0).SetBytes(x), nil
		})).
	Complete()

var cborAtlas atlas.Atlas
var cborSortingMode = atlas.KeySortMode_RFC7049
var atlasEntries = []*atlas.AtlasEntry{cidAtlasEntry, bigIntAtlasEntry}

var (
	numWorkers   = runtime.NumCPU() + 1
	cloner       = encoding.PooledCloner{Count: numWorkers}
	unmarshaller = encoding.PooledUnmarshaller{Count: numWorkers}
	marshaller   = encoding.PooledMarshaller{Count: numWorkers}
)

func init() {
	rebuildAtlas()
}

func rebuildAtlas() {
	cborAtlas = atlas.MustBuild(atlasEntries...).
		WithMapMorphism(atlas.MapMorphism{atlas.KeySortMode_RFC7049})

	marshaller.SetAtlas(cborAtlas)
	unmarshaller.SetAtlas(cborAtlas)
	cloner.SetAtlas(cborAtlas)
}

// RegisterCborType allows to register a custom cbor type
func RegisterCborType(i interface{}) {
	var entry *atlas.AtlasEntry
	if ae, ok := i.(*atlas.AtlasEntry); ok {
		entry = ae
	} else {
		entry = atlas.BuildEntry(i).StructMap().AutogenerateWithSortingScheme(atlas.KeySortMode_RFC7049).Complete()
	}
	atlasEntries = append(atlasEntries, entry)
	rebuildAtlas()
}
