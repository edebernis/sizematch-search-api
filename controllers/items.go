package controllers

import (
    "encoding/json"
    "fmt"
    elasticsearch "github.com/elastic/go-elasticsearch/v8"
    "github.com/gin-gonic/gin"
    "io"
    "net/http"
    "reflect"
    "strings"
)

// ItemsController ...
type ItemsController struct {
    IndexName string
}

// Item ...
type Item struct {
    ID          string                      `json:"id"`
    Score       float64                     `json:"score"`
    Source      string                      `json:"source"`
    Timestamp   uint64                      `json:"timestamp"`
    Name        string                      `json:"name"`
    Description string                      `json:"description"`
    Urls        []string                    `json:"urls"`
    Categories  []string                    `json:"categories"`
    ImageUrls   []string                    `json:"image_urls"`
    Dimensions  map[esItemDimension]float64 `json:"dimensions"`
    Price       price                       `json:"price"`
}

type price struct {
    Amount   float64 `json:"amount"`
    Currency string  `json:"currency"`
}

// ESItem ...
type ESItem struct {
    Source     string                      `json:"source"`
    Timestamp  uint64                      `json:"timestamp"`
    ImageUrls  []string                    `json:"image_urls"`
    Dimensions map[esItemDimension]float64 `json:"dimensions"`
    Name       struct {
        EN string `json:"en"`
        FR string `json:"fr"`
    }
    Description struct {
        EN string `json:"en"`
        FR string `json:"fr"`
    }
    Urls struct {
        EN []string `json:"en"`
        FR []string `json:"fr"`
    }
    Categories struct {
        EN []string `json:"en"`
        FR []string `json:"fr"`
    }
    Price struct {
        EN price `json:"en"`
        FR price `json:"fr"`
    }
}

type esItemDimension string

const (
    esLength    esItemDimension = "length"
    esHeight    esItemDimension = "height"
    esWidth     esItemDimension = "width"
    esDepth     esItemDimension = "depth"
    esWeight    esItemDimension = "weight"
    esDiameter  esItemDimension = "diameter"
    esVolume    esItemDimension = "volume"
    esThickness esItemDimension = "thickness"
)

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

type esDimensionFilter struct {
    dimension esItemDimension
    filter    string
}

var itemsSearchParamsDimensionFiltersMap = map[string]esDimensionFilter{
    "MinLength":    {dimension: esLength, filter: "gte"},
    "MaxLength":    {dimension: esLength, filter: "lte"},
    "MinHeight":    {dimension: esHeight, filter: "gte"},
    "MaxHeight":    {dimension: esHeight, filter: "lte"},
    "MinWidth":     {dimension: esWidth, filter: "gte"},
    "MaxWidth":     {dimension: esWidth, filter: "lte"},
    "MinDepth":     {dimension: esDepth, filter: "gte"},
    "MaxDepth":     {dimension: esDepth, filter: "lte"},
    "MinWeight":    {dimension: esWeight, filter: "gte"},
    "MaxWeight":    {dimension: esWeight, filter: "lte"},
    "MinDiameter":  {dimension: esDiameter, filter: "gte"},
    "MaxDiameter":  {dimension: esDiameter, filter: "lte"},
    "MinVolume":    {dimension: esVolume, filter: "gte"},
    "MaxVolume":    {dimension: esVolume, filter: "lte"},
    "MinThickness": {dimension: esThickness, filter: "gte"},
    "MaxThickness": {dimension: esThickness, filter: "lte"},
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

    results.Total = r.Hits.Total.Value

    if len(r.Hits.Hits) < 1 {
        results.Items = []*Item{}
        return &results, nil
    }

    for _, hit := range r.Hits.Hits {
        i := Item{
            ID:    hit.ID,
            Score: hit.Score,
        }

        var esi ESItem
        if err := json.Unmarshal(hit.Source, &esi); err != nil {
            return &results, err
        }

        i.Source = esi.Source
        i.Timestamp = esi.Timestamp
        i.ImageUrls = esi.ImageUrls
        i.Dimensions = esi.Dimensions
        i.Name = ctrl.getESItemValueByLang(esi.Name, p.Lang).String()
        i.Description = ctrl.getESItemValueByLang(esi.Description, p.Lang).String()
        i.Urls = ctrl.getESItemValueByLang(esi.Urls, p.Lang).Interface().([]string)
        i.Categories = ctrl.getESItemValueByLang(esi.Categories, p.Lang).Interface().([]string)
        i.Price = ctrl.getESItemValueByLang(esi.Price, p.Lang).Interface().(price)

        results.Items = append(results.Items, &i)
    }

    return &results, nil
}

func (ctrl *ItemsController) getESItemValueByLang(i interface{}, lang string) reflect.Value {
    return reflect.ValueOf(i).FieldByName(strings.ToUpper(lang))
}

func (ctrl *ItemsController) buildSearchQuery(p *itemsSearchParams) io.Reader {
    var b strings.Builder

    filters := ctrl.buildDimensionsFilter(p)

    b.WriteString("{")
    b.WriteString(fmt.Sprintf(esQuery, p.Query, p.Lang, p.Lang, p.Lang, filters, p.Lang))

    if len(p.After) > 0 {
        b.WriteString(",\n")
        b.WriteString(fmt.Sprintf(`    "search_after": [%s]`, p.After))
    }

    b.WriteString("\n}")

    //fmt.Printf("%s\n", b.String())
    return strings.NewReader(b.String())
}

func (ctrl *ItemsController) buildDimensionsFilter(p *itemsSearchParams) string {
    var b strings.Builder

    for _, searchParamName := range reflect.ValueOf(itemsSearchParamsDimensionFiltersMap).MapKeys() {
        searchParamValue := reflect.ValueOf(p).FieldByName(searchParamName.String()).Float()
        if searchParamValue != 0 {
            obj := itemsSearchParamsDimensionFiltersMap[searchParamName.String()]
            b.WriteString(fmt.Sprintf(esQueryFilter, obj.dimension, obj.filter, searchParamValue))
        }
    }

    s := b.String()
    if len(s) > 0 {
        s = s[:len(s)-2] // Remove trailing comma and newline characters
    }

    return s
}

const esQuery = `
    "query" : {
        "bool": {
            "must": [{
                "multi_match" : {
                    "query" : %q,
                    "fields" : ["name.%s^10", "categories.%s^3", "description.%s"],
                    "operator" : "and"
                }
            }],
            "filter": [
%s
            ]
        }
    },
    "size" : 25,
    "sort" : [ { "_score" : "desc" }, { "timestamp" : "asc" } ]`

const esQueryFilter = `                { "range":  { "dimensions.%s": {"%s": %f} }},
`
