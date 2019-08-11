package rest

import (
	"log"
	"net/http"
	"net/url"
	"strconv"

	"github.com/HDIOES/cpa-backend/models"
)

//GenreHandler struct
type GenreHandler struct {
	Dao *models.GenreDAO
}

func (g *GenreHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	vars, parseErr := url.ParseQuery(r.URL.RawQuery)
	if parseErr != nil {
		log.Println(parseErr)
	}
	genreSQLBuilder := models.GenreQueryBuilder{}
	if limit, limitOk := vars["limit"]; limitOk {
		limitInt64, parseErr := strconv.ParseInt(limit[0], 10, 32)
		if parseErr != nil {
			HandleErr(parseErr, w, 400, "Not valid limit")
			return
		}
		genreSQLBuilder.SetOffset(int32(limitInt64))
	}
	if offset, offsetOk := vars["offset"]; offsetOk {
		offsetInt64, parseErr := strconv.ParseInt(offset[0], 10, 32)
		if parseErr != nil {
			HandleErr(parseErr, w, 400, "Not valid offset")
			return
		}
		genreSQLBuilder.SetOffset(int32(offsetInt64))

	}
	genreDtos, findByFilterErr := g.Dao.FindByFilter(genreSQLBuilder)
	if findByFilterErr != nil {
		HandleErr(findByFilterErr, w, 400, "Error")
		return
	}
	genres := []GenreRo{}
	for _, genreDto := range genreDtos {
		genreRo := GenreRo{}
		genreRo.ID = genreDto.ExternalID
		genreRo.Name = genreDto.Name
		genreRo.Russian = genreDto.Russian
		genreRo.Kind = genreDto.Kind
		genres = append(genres, genreRo)
	}
	ReturnResponseAsJSON(w, genres, 200)
}

//GenreRo struct
type GenreRo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Russian string `json:"russian"`
	Kind    string `json:"kind"`
}