package controllers

import (
    "encoding/json"
    "fmt"
    elasticsearch "github.com/elastic/go-elasticsearch/v8"
    "github.com/gin-gonic/gin"
    "io"
    "net/http"
    "strings"
)

// ItemsController ...
type ItemsController struct {
    IndexName string
}

// Item ...
type Item struct {
    ID     string `json:"id"`
    Source string `json:"source"`
}

// ItemsSearchResults ...
type ItemsSearchResults struct {
    Total int     `json:"total"`
    Items []*Item `json:"items"`
}

type itemsSearchParams struct {
    Query        string  `form:"q" binding:"required"`
    After        string  `form:"a"`
    Lang         string  `form:"lang,default=en"`
    MinLength    float64 `form:"min_length"`
    MaxLength    float64 `form:"max_length"`
    MinHeight    float64 `form:"min_height"`
    MaxHeight    float64 `form:"max_height"`
    MinWidth     float64 `form:"min_width"`
    MaxWidth     float64 `form:"max_width"`
    MinDepth     float64 `form:"min_depth"`
    MaxDepth     float64 `form:"max_depth"`
    MinWeight    float64 `form:"min_weight"`
    MaxWeight    float64 `form:"max_weight"`
    MinDiameter  float64 `form:"min_diameter"`
    MaxDiameter  float64 `form:"max_diameter"`
    MinThickness float64 `form:"min_thickness"`
    MaxThickness float64 `form:"max_thickness"`
    MinVolume    float64 `form:"min_volume"`
    MaxVolume    float64 `form:"max_volume"`
}

// RoutesV1 setups V1 items routes
func (ctrl *ItemsController) RoutesV1(g *gin.RouterGroup) {
    g.GET("/items", ctrl.searchRoute)
    g.OPTIONS("/items", ctrl.optionsRoute)
}

func (ctrl *ItemsController) searchRoute(c *gin.Context) {
    es := c.MustGet("es").(*elasticsearch.Client)

    var p itemsSearchParams
    if err := c.ShouldBindQuery(&p); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"msg": "Missing required parameters"})
        return
    }

    results, err := ctrl.search(es, &p)
    if err != nil {
        fmt.Println(err)
        c.JSON(http.StatusInternalServerError, gin.H{"msg": "Internal Server Error"})
        return
    }

    c.JSON(http.StatusOK, results)
}

func (ctrl *ItemsController) optionsRoute(c *gin.Context) {
    c.Writer.Header().Set("Access-Control-Allow-Methods", "GET")
    c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
    c.Next()
}

func (ctrl *ItemsController) search(es *elasticsearch.Client, p *itemsSearchParams) (*ItemsSearchResults, error) {
    var results ItemsSearchResults

    query := ctrl.buildSearchQuery(p)

    res, err := es.Search(
        es.Search.WithIndex(ctrl.IndexName),
        es.Search.WithBody(query),
    )
    if err != nil {
        return &results, err
    }
    defer res.Body.Close()

    if res.IsError() {
        return &results, fmt.Errorf("%s", res.Body)
    }

    var r esEnvelopeResponse
    if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
        return &results, err
    }

    if len(r.Hits.Hits) < 1 {
        results.Items = []*Item{}
        return &results, nil
    }

    for _, hit := range r.Hits.Hits {
        var h Item
        h.ID = hit.ID

        if err := json.Unmarshal(hit.Source, &h); err != nil {
            return &results, err
        }

        results.Items = append(results.Items, &h)
    }

    return &results, nil
}

func (ctrl *ItemsController) buildSearchQuery(p *itemsSearchParams) io.Reader {
    var b strings.Builder

    b.WriteString("{")
    b.WriteString(fmt.Sprintf(searchMatch, p.Query, p.Lang, p.Lang))

    if len(p.After) > 0 {
        b.WriteString(",\n")
        b.WriteString(fmt.Sprintf(` "search_after": %s`, p.After))
    }

    b.WriteString("\n}")

    fmt.Printf("%s\n", b.String())
    return strings.NewReader(b.String())
}

const searchMatch = `
    "query" : {
        "multi_match" : {
            "query" : %q,
            "fields" : ["name.%s^10", "description.%s"],
            "operator" : "and"
        }
    },
    "size" : 25,
    "sort" : [ { "_score" : "desc" }, { "_doc" : "asc" } ]`
