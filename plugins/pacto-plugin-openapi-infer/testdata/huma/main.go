package main

import (
	"github.com/danielgtaylor/huma/v2"
)

type Pet struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Tag  string `json:"tag"`
}

type PetCreate struct {
	Name string `json:"name"`
	Tag  string `json:"tag"`
}

func registerRoutes(api huma.API) {
	cfg := huma.DefaultConfig("Pet Store", "1.0.0")
	_ = cfg

	huma.Get(api, "/pets", listPets)
	huma.Get(api, "/pets/{petId}", getPet)
	huma.Post(api, "/pets", createPet)
	huma.Delete(api, "/pets/{petId}", deletePet)
}
