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
    ID          string             `json:"id"`
    Score       float64            `json:"score"`
    Source      string             `json:"source"`
    Timestamp   uint64             `json:"timestamp"`
    Name        string             `json:"name"`
    Description string             `json:"description"`
    Urls        []string           `json:"urls"`
    Categories  []string           `json:"categories"`
    ImageUrls   []string           `json:"image_urls"`
    Dimensions  map[string]float64 `json:"dimensions"`
    Price       price              `json:"price"`
}

type price struct {
    Amount   float64 `json:"amount"`
    Currency string  `json:"currency"`
}

// ESItem ...
type ESItem struct {
    Source     string             `json:"source"`
    Timestamp  uint64             `json:"timestamp"`
    ImageUrls  []string           `json:"image_urls"`
    Dimensions map[string]float64 `json:"dimensions"`
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
        i.Name = reflect.ValueOf(esi.Name).FieldByName(strings.ToUpper(p.Lang)).String()
        i.Description = reflect.ValueOf(esi.Description).FieldByName(strings.ToUpper(p.Lang)).String()
        i.Urls = reflect.ValueOf(esi.Urls).FieldByName(strings.ToUpper(p.Lang)).Interface().([]string)
        i.Categories = reflect.ValueOf(esi.Categories).FieldByName(strings.ToUpper(p.Lang)).Interface().([]string)
        i.Price = reflect.ValueOf(esi.Price).FieldByName(strings.ToUpper(p.Lang)).Interface().(price)

        results.Items = append(results.Items, &i)
    }

    return &results, nil
}

func (ctrl *ItemsController) buildSearchQuery(p *itemsSearchParams) io.Reader {
    var b strings.Builder

    filters := ctrl.buildDimensionsFilter(p)

    b.WriteString("{")
    b.WriteString(fmt.Sprintf(searchMatch, p.Query, p.Lang, p.Lang, filters))

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

    if p.MinLength != 0 {
        b.WriteString(fmt.Sprintf(dimensionsFilter, "length", "gte", p.MinLength))
    }
    if p.MaxLength != 0 {
        b.WriteString(fmt.Sprintf(dimensionsFilter, "length", "lte", p.MaxLength))
    }
    if p.MinHeight != 0 {
        b.WriteString(fmt.Sprintf(dimensionsFilter, "height", "gte", p.MinHeight))
    }
    if p.MaxHeight != 0 {
        b.WriteString(fmt.Sprintf(dimensionsFilter, "height", "lte", p.MaxHeight))
    }
    if p.MinWidth != 0 {
        b.WriteString(fmt.Sprintf(dimensionsFilter, "width", "gte", p.MinWidth))
    }
    if p.MaxWidth != 0 {
        b.WriteString(fmt.Sprintf(dimensionsFilter, "width", "lte", p.MaxWidth))
    }
    if p.MinDepth != 0 {
        b.WriteString(fmt.Sprintf(dimensionsFilter, "depth", "gte", p.MinDepth))
    }
    if p.MaxDepth != 0 {
        b.WriteString(fmt.Sprintf(dimensionsFilter, "depth", "lte", p.MaxDepth))
    }
    if p.MinWeight != 0 {
        b.WriteString(fmt.Sprintf(dimensionsFilter, "weight", "gte", p.MinWeight))
    }
    if p.MaxWeight != 0 {
        b.WriteString(fmt.Sprintf(dimensionsFilter, "weight", "lte", p.MaxWeight))
    }
    if p.MinDiameter != 0 {
        b.WriteString(fmt.Sprintf(dimensionsFilter, "diameter", "gte", p.MinDiameter))
    }
    if p.MaxDiameter != 0 {
        b.WriteString(fmt.Sprintf(dimensionsFilter, "diameter", "lte", p.MaxDiameter))
    }
    if p.MinVolume != 0 {
        b.WriteString(fmt.Sprintf(dimensionsFilter, "volume", "gte", p.MinVolume))
    }
    if p.MaxVolume != 0 {
        b.WriteString(fmt.Sprintf(dimensionsFilter, "volume", "lte", p.MaxVolume))
    }
    if p.MinThickness != 0 {
        b.WriteString(fmt.Sprintf(dimensionsFilter, "thickness", "gte", p.MinThickness))
    }
    if p.MaxThickness != 0 {
        b.WriteString(fmt.Sprintf(dimensionsFilter, "thickness", "lte", p.MaxThickness))
    }

    s := b.String()
    if len(s) > 0 {
        s = s[:len(s)-2] // Remove trailing comma and newline characters
    }

    return s
}

const searchMatch = `
    "query" : {
        "bool": {
            "must": [{
                "multi_match" : {
                    "query" : %q,
                    "fields" : ["name.%s^10", "description.%s"],
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

const dimensionsFilter = `                { "range":  { "dimensions.%s": {"%s": %f} }},
`
