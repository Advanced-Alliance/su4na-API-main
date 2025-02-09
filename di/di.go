package di

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/HDIOES/su4na-API-main/integration"
	"github.com/HDIOES/su4na-API-main/models"
	"github.com/HDIOES/su4na-API-main/rest"
	"github.com/HDIOES/su4na-API-main/rest/util"
	"github.com/gorilla/mux"
	loghttp "github.com/motemen/go-loghttp"
	"github.com/ory/dockertest"
	"github.com/pkg/errors"
	migrate "github.com/rubenv/sql-migrate"
	"github.com/tkanos/gonfig"
	"go.uber.org/dig"
)

const dbType = "postgres"
const dbVersion = "11.4"
const dbPortMapping = "5432/tcp"

const dbUser = "test_forna_user"
const dbUserVar = "POSTGRES_USER=" + dbUser

const dbPassword = "12345"
const dbPasswordVar = "POSTGRES_PASSWORD=" + dbPassword

const dbName = "test_forna"
const dbNameVar = "POSTGRES_DB=" + dbName

const dbURLTemplate = dbType + "://" + dbUser + ":" + dbPassword + "@localhost:%s/" + dbName + "?sslmode=disable"

//CreateDI function to build di-container
func CreateDI(configPath, migrationPath string, test bool) *dig.Container {
	container := dig.New()
	container.Provide(func() *util.Configuration {
		log.Println("Loading configuration...")
		configuration := &util.Configuration{}
		gonfigErr := gonfig.GetConf(configPath, configuration)
		if gonfigErr != nil {
			panic(gonfigErr)
		}
		return configuration
	})
	container.Provide(func(configuration *util.Configuration) (sqlDb *sql.DB, dockerResource *dockertest.Resource, err error) {
		if test {
			pool, err := dockertest.NewPool("")
			if err != nil {
				return nil, nil, errors.Wrap(err, "")
			}
			resource, rErr := pool.Run(dbType, dbVersion, []string{
				dbUserVar,
				dbPasswordVar,
				dbNameVar})
			log.Print("Starting test container...")
			time.Sleep(10 * time.Second)
			if rErr != nil {
				defer resource.Close()
				return nil, nil, errors.Wrap(rErr, "")
			}
			configuration.DatabaseURL = fmt.Sprintf(dbURLTemplate, resource.GetPort(dbPortMapping))
			log.Println("PORT = " + resource.GetPort(dbPortMapping))
			dockerResource = resource
		}
		log.Println("Prepating db...")
		db, err := sql.Open("postgres", configuration.DatabaseURL)
		if err != nil {
			panic(err)
		}
		db.SetMaxIdleConns(configuration.MaxIdleConnections)
		db.SetMaxOpenConns(configuration.MaxOpenConnections)
		timeout := strconv.Itoa(configuration.ConnectionTimeout) + "s"
		timeoutDuration, durationErr := time.ParseDuration(timeout)
		if durationErr != nil {
			log.Println("Error parsing of timeout parameter")
			panic(durationErr)
		} else {
			db.SetConnMaxLifetime(timeoutDuration)
		}
		migrations := &migrate.FileMigrationSource{
			Dir: migrationPath,
		}
		if n, err := migrate.Exec(db, "postgres", migrations, migrate.Up); err == nil {
			log.Printf("Applied %d migrations!\n", n)
		} else {
			return nil, nil, errors.Wrap(err, "")
		}
		sqlDb = db
		return
	})
	container.Provide(func(db *sql.DB) (*models.AnimeDAO, *models.GenreDAO, *models.StudioDAO, *models.NewDAO) {
		return &models.AnimeDAO{Db: db}, &models.GenreDAO{Db: db}, &models.StudioDAO{Db: db}, &models.NewDAO{Db: db}
	})
	container.Provide(func(configuration *util.Configuration) *integration.ShikimoriDao {
		transport := &loghttp.Transport{
			LogRequest: func(req *http.Request) {
				var loggedHTTPBody []byte
				reqBodyReadCloser := req.Body
				if reqBodyReadCloser != nil {
					defer reqBodyReadCloser.Close()
					httpBody, err := ioutil.ReadAll(req.Body)
					if err != nil {
						log.Println("Cannot read request body")
					} else {
						loggedHTTPBody = httpBody
					}
				}
				loggedHTTPBodyValue := ""
				if loggedHTTPBody != nil {
					loggedHTTPBodyValue = string(loggedHTTPBody)
					req.Body = ioutil.NopCloser(bytes.NewReader(loggedHTTPBody))
				}
				log.Printf("Http request: URL: %v Method: %v Headers: %v Body: %v", req.URL.String(), req.Method, req.Header, loggedHTTPBodyValue)

			},
			LogResponse: func(resp *http.Response) {
				var loggedHTTPBody []byte
				respBodyReadCloser := resp.Body
				if respBodyReadCloser != nil {
					defer respBodyReadCloser.Close()
					httpBody, err := ioutil.ReadAll(respBodyReadCloser)
					if err != nil {
						log.Println("Cannot read response body")
					} else {
						loggedHTTPBody = httpBody
					}
				}
				loggedHTTPBodyValue := ""
				if loggedHTTPBody != nil {
					loggedHTTPBodyValue = string(loggedHTTPBody)
					resp.Body = ioutil.NopCloser(bytes.NewReader(loggedHTTPBody))
				}
				log.Printf("Http response: Status: %v Headers: %v Body: %v", resp.StatusCode, resp.Header, loggedHTTPBodyValue)
			},
		}
		client := &http.Client{
			Transport: transport,
		}
		shikimoriDao := integration.ShikimoriDao{Client: client, Config: configuration}
		return &shikimoriDao
	})
	container.Provide(func(animeDao *models.AnimeDAO, genreDao *models.GenreDAO, studioDao *models.StudioDAO, configuration *util.Configuration, shikimoriDao *integration.ShikimoriDao) *integration.ShikimoriJob {
		log.Println("Prepare shikimori job")
		return &integration.ShikimoriJob{AnimeDao: animeDao, GenreDao: genreDao, StudioDao: studioDao, Config: configuration, ShikimoriDao: shikimoriDao}
	})
	container.Provide(func(
		animeDao *models.AnimeDAO,
		genreDao *models.GenreDAO,
		studioDao *models.StudioDAO,
		newDao *models.NewDAO,
		configuration *util.Configuration) (
		*rest.GenreHandler,
		*rest.CreateNewHandler,
		*rest.FindNewHandler,
		*rest.RandomAnimeHandler,
		*rest.StudioHandler,
		*rest.SearchAnimeHandler) {
		genreHandler := &rest.GenreHandler{Dao: genreDao}
		createNewHandler := &rest.CreateNewHandler{Dao: newDao}
		findNewHandler := &rest.FindNewHandler{Dao: newDao}
		randomAnimeHandler := &rest.RandomAnimeHandler{Dao: animeDao, Configuration: configuration}
		studioHandler := &rest.StudioHandler{Dao: studioDao}
		searchAnimeHandler := &rest.SearchAnimeHandler{Dao: animeDao, Configuration: configuration}
		return genreHandler, createNewHandler, findNewHandler, randomAnimeHandler, studioHandler, searchAnimeHandler
	})
	container.Provide(func(genreHandler *rest.GenreHandler,
		createNewHandler *rest.CreateNewHandler,
		findNewHandler *rest.FindNewHandler,
		randomAnimeHandler *rest.RandomAnimeHandler,
		studioHandler *rest.StudioHandler,
		searchAnimeHandler *rest.SearchAnimeHandler) *mux.Router {
		router := mux.NewRouter()
		router.Handle("/api/animes/random", randomAnimeHandler).
			Methods("GET")
		router.Handle("/api/animes/search", searchAnimeHandler).
			Methods("GET")
		router.Handle("/api/genres/search", genreHandler).
			Methods("GET")
		router.Handle("/api/studios/search", studioHandler).
			Methods("GET")
		router.Handle("/api/news", createNewHandler).
			Methods("POST")
		router.Handle("/api/news", findNewHandler).
			Methods("GET")
		router.Handle("/api/news", nil).
			Methods("DELETE")
		http.Handle("/", router)
		return router
	})
	return container
}
