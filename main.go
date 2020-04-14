package main

import (
    "github.com/edebernis/sizematch-search-api/controllers"
    elasticsearch "github.com/elastic/go-elasticsearch/v8"
    "github.com/gin-gonic/gin"
    "log"
    "os"
    "strings"
)

func getEnv(key, fallback string) string {
    if value, ok := os.LookupEnv(key); ok {
        return value
    }
    return fallback
}

func es() gin.HandlerFunc {
    cfg := elasticsearch.Config{
        Addresses: strings.Split(getEnv("ELASTICSEARCH_URLS", "http://localhost:9200"), ","),
        Username:  getEnv("ELASTICSEARCH_USERNAME", ""),
        Password:  getEnv("ELASTICSEARCH_PASSWORD", ""),
    }

    es, err := elasticsearch.NewClient(cfg)
    if err != nil {
        log.Fatalf("Error creating the ES client: %s", err)
    }

    return func(c *gin.Context) {
        c.Set("es", es)
        c.Next()
    }
}

func cors() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Writer.Header().Add("Access-Control-Allow-Origin", "*")
        c.Next()
    }
}

func setup(router *gin.Engine) {
    // Init controllers
    items := controllers.ItemsController{
        IndexName: os.Getenv("ITEMS_INDEX"),
    }

    // Init routes
    v1 := router.Group("/v1")
    items.RoutesV1(v1)
}

func main() {
    router := gin.Default()

    router.Use(cors())
    router.Use(es())

    setup(router)
    router.Run()
}
