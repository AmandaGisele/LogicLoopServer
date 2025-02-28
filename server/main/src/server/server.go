package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/schollz/find3/server/main/src/api"
	"github.com/schollz/find3/server/main/src/database"
	"github.com/schollz/find3/server/main/src/models"
	"github.com/schollz/find3/server/main/src/mqtt"
	"github.com/schollz/utils"
	"github.com/rs/cors"
)

// Port defines the public port
var Port = "8003"
var UseSSL = false
var UseMQTT = false
var MinimumPassive = -1

func RunWithCORSAndHTTPS() error {
	// Define your routes and handlers here
	mux := http.NewServeMux()
	// Example: mux.HandleFunc("/api/v1/by_location/", YourHandler)

	// Enable CORS
	handler := cors.Default().Handler(mux)

	// Start the HTTPS server
	return http.ListenAndServeTLS(":8003", "/etc/ssl/fullchain.pem", "/etc/ssl/privkey.pem", handler)
}

// Run will start the server listening on the specified port
func Run() (err error) {
	defer logger.Log.Flush()

	if UseMQTT {
		// setup MQTT
		err = mqtt.Setup()
		if err != nil {
			logger.Log.Warn(err)
		}
		logger.Log.Debug("setup mqtt")
	}

	logger.Log.Debug("current families: ", database.GetFamilies())

	// setup gin server
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	// Standardize logs
	r.LoadHTMLGlob("templates/*")
	r.Static("/static", "./static")
	r.Use(middleWareHandler(), gin.Recovery(), gzip.Gzip(gzip.DefaultCompression))
	// r.Use(middleWareHandler(), gin.Recovery())
	r.HEAD("/", func(c *gin.Context) { // handler for the uptime robot
		c.String(http.StatusOK, "OK")
	})
	r.GET("/", func(c *gin.Context) { // handler for the uptime robot
		c.HTML(http.StatusOK, "login.tmpl", gin.H{
			"Message": "",
		})
	})
	r.POST("/", func(c *gin.Context) {
		family := strings.ToLower(c.PostForm("inputFamily"))
		db, err := database.Open(family, true)
		if err == nil {
			db.Close()
			c.Redirect(http.StatusMovedPermanently, "/view/dashboard/"+family)
		} else {
			c.HTML(http.StatusOK, "login.tmpl", gin.H{
				"Message": template.HTML(fmt.Sprintf(`Family '%s' does not exist. Follow <a href="https://www.internalpositioning.com/doc/tracking_your_phone.md" target="_blank">these instructions</a> to get started.`, family)),
			})
		}
	})
	r.DELETE("/api/v1/database/:family", func(c *gin.Context) {
		family := strings.ToLower(c.Param("family"))
		db, err := database.Open(family, true)
		if err == nil {
			db.Delete()
			db.Close()
			c.JSON(200, gin.H{"success": true, "message": "deleted " + family})
		} else {
			c.JSON(200, gin.H{"success": false, "message": err.Error()})
		}
	})
	r.DELETE("/api/v1/location/:family/:location", func(c *gin.Context) {
		family := strings.ToLower(c.Param("family"))
		db, err := database.Open(family, true)
		if err == nil {
			err = db.DeleteLocation(c.Param("location"))
			db.Close()
			if err == nil {
				c.JSON(200, gin.H{"success": true, "message": "deleted location '" + c.Param("location") + "' for " + family})
				return
			}
		}
		c.JSON(200, gin.H{"success": false, "message": err.Error()})
	})
	r.GET("/view/analysis/:family", func(c *gin.Context) {
		family := strings.ToLower(c.Param("family"))
		d, err := database.Open(family, true)
		if err != nil {
			c.String(200, err.Error())
			return
		}
		locationList, err := d.GetLocations()
		d.Close()
		if err != nil {
			logger.Log.Warn("could not get locations")
			c.String(200, err.Error())
			return
		}
		c.HTML(http.StatusOK, "analysis.tmpl", gin.H{
			"LocationAnalysis": true,
			"Family":           family,
			"Locations":        locationList,
			"FamilyJS":         template.JS(family),
		})
	})
	r.GET("/view/location_analysis/:family/:location", func(c *gin.Context) {
		family := strings.ToLower(c.Param("family"))
		img, err := api.GetImage(family, c.Param("location"))
		if err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("unable to locate image for '%s' for '%s'", c.Param("location"), family))
		} else {
			c.Data(200, "image/png", img)
		}
	})
	r.GET("/view/location/:family/:device", func(c *gin.Context) {
		family := strings.ToLower(c.Param("family"))
		device := c.Param("device")
		c.HTML(http.StatusOK, "location.tmpl", gin.H{
			"Family":   family,
			"Device":   device,
			"FamilyJS": template.JS(family),
			"DeviceJS": template.JS(device),
		})
	})
	r.GET("/view/map2/:family", func(c *gin.Context) {
		family := strings.ToLower(c.Param("family"))

		err := func(family string) (err error) {
			gpsData, err := api.GetGPSData(family)
			if err != nil {
				return
			}

			// initialize GPS data
			type gpsdata struct {
				Hash      template.JS
				Location  template.JS
				Latitude  template.JS
				Longitude template.JS
			}
			data := make([]gpsdata, len(gpsData))
			avgLat := 0.0
			avgLon := 0.0
			i := 0
			for loc := range gpsData {
				data[i].Hash = template.JS(utils.Md5Sum(loc))
				data[i].Location = template.JS(loc)
				latitude := 0.0
				longitude := 0.0
				if _, ok := gpsData[loc]; ok {
					latitude = gpsData[loc].GPS.Latitude
					longitude = gpsData[loc].GPS.Longitude
				}
				avgLat += latitude
				avgLon += longitude
				data[i].Latitude = template.JS(fmt.Sprintf("%2.10f", latitude))
				data[i].Longitude = template.JS(fmt.Sprintf("%2.10f", longitude))
				i++
			}
			avgLat = avgLat / float64(len(gpsData))
			avgLon = avgLon / float64(len(gpsData))

			c.HTML(200, "map2.tmpl", gin.H{
				"UserMap":  true,
				"Family":   family,
				"Device":   "all",
				"FamilyJS": template.JS(family),
				"DeviceJS": template.JS("all"),
				"Data":     data,
				"Center":   template.JS(fmt.Sprintf("%2.5f,%2.5f", avgLat, avgLon)),
			})
			return
		}(family)
		if err != nil {
			logger.Log.Warn(err)
			c.HTML(200, "map2.tmpl", gin.H{
				"UserMap":      true,
				"ErrorMessage": err.Error(),
				"Family":       family,
				"Device":       "all",
				"FamilyJS":     template.JS(family),
				"DeviceJS":     template.JS("all"),
			})
		}
	})
	r.GET("/view/map/:family", func(c *gin.Context) {
		family := strings.ToLower(c.Param("family"))
		err := func(family string) (err error) {
			gpsData, err := api.GetGPSData(family)
			if err != nil {
				return
			}

			// initialize GPS data
			type gpsdata struct {
				Hash      template.JS
				Location  template.JS
				Latitude  template.JS
				Longitude template.JS
			}
			data := make([]gpsdata, len(gpsData))
			avgLat := 0.0
			avgLon := 0.0
			i := 0
			for loc := range gpsData {
				data[i].Hash = template.JS(utils.Md5Sum(loc))
				data[i].Location = template.JS(loc)
				latitude := 0.0
				longitude := 0.0
				if _, ok := gpsData[loc]; ok {
					latitude = gpsData[loc].GPS.Latitude
					longitude = gpsData[loc].GPS.Longitude
				}
				avgLat += latitude
				avgLon += longitude
				data[i].Latitude = template.JS(fmt.Sprintf("%2.10f", latitude))
				data[i].Longitude = template.JS(fmt.Sprintf("%2.10f", longitude))
				i++
			}
			avgLat = avgLat / float64(len(gpsData))
			avgLon = avgLon / float64(len(gpsData))

			c.HTML(200, "map.tmpl", gin.H{
				"Map":    true,
				"Family": family,
				"Data":   data,
				"Center": template.JS(fmt.Sprintf("%2.5f,%2.5f", avgLat, avgLon)),
			})
			return
		}(family)
		if err != nil {
			logger.Log.Warn(err)
			c.HTML(200, "map.tmpl", gin.H{
				"Map":          true,
				"ErrorMessage": err.Error(),
				"Family":       family,
			})
		}
	})
	r.GET("/api/v1/database/:family", func(c *gin.Context) {
		db, err := database.Open(strings.ToLower(c.Param("family")), true)
		if err == nil {
			var dumped string
			dumped, err = db.Dump()
			db.Close()
			if err == nil {
				c.String(200, dumped)
				return
			}
		}
		c.JSON(200, gin.H{"success": false, "message": err.Error()})
	})
	r.GET("/api/v1/data/:family", func(c *gin.Context) {
		var sensors []models.SensorData
		var message string
		db, err := database.Open(strings.ToLower(c.Param("family")), true)
		if err == nil {
			sensors, err = db.GetAllForClassification()
			db.Close()
		}
		if err != nil {
			message = err.Error()
		} else {
			message = fmt.Sprintf("got %d data", len(sensors))
		}
		c.JSON(200, gin.H{"success": err == nil, "message": message, "data": sensors})
	})
	r.GET("/view/gps/:family", func(c *gin.Context) {
		err := func(family string) (err error) {
			logger.Log.Debugf("[%s] getting gps", family)
			gpsData, err := api.GetGPSData(family)
			if err != nil {
				return
			}

			// initialize GPS data
			type gpsdata struct {
				Hash      template.JS
				Location  template.JS
				Latitude  template.JS
				Longitude template.JS
			}
			data := make([]gpsdata, len(gpsData))
			avgLat := 0.0
			avgLon := 0.0
			i := 0
			for loc := range gpsData {
				data[i].Hash = template.JS(utils.Md5Sum(loc))
				data[i].Location = template.JS(loc)
				latitude := 0.0
				longitude := 0.0
				if _, ok := gpsData[loc]; ok {
					latitude = gpsData[loc].GPS.Latitude
					longitude = gpsData[loc].GPS.Longitude
				}
				avgLat += latitude
				avgLon += longitude
				data[i].Latitude = template.JS(fmt.Sprintf("%2.10f", latitude))
				data[i].Longitude = template.JS(fmt.Sprintf("%2.10f", longitude))
				i++
			}
			avgLat = avgLat / float64(len(gpsData))
			avgLon = avgLon / float64(len(gpsData))

			c.HTML(200, "gps.tmpl", gin.H{
				"Family": family,
				"Data":   data,
				"Center": template.JS(fmt.Sprintf("%2.5f,%2.5f", avgLat, avgLon)),
			})
			return
		}(strings.ToLower(c.Param("family")))
		if err != nil {
			c.String(403, err.Error())
		}
	})
	r.GET("/view/dashboard/:family", func(c *gin.Context) {
		type LocEff struct {
			Name           string
			Total          int64
			PercentCorrect int64
		}
		type Efficacy struct {
			AccuracyBreakdown   []LocEff
			LastCalibrationTime time.Time
			TotalCount          int64
			PercentCorrect      int64
		}
		type DeviceTable struct {
			ID           string
			Name         string
			LastLocation string
			LastSeen     time.Time
			Probability  int64
			ActiveTime   int64
		}

		family := strings.ToLower(c.Param("family"))
		err := func(family string) (err error) {
			startTime := time.Now()
			var errorMessage string

			d, err := database.Open(family, true)
			if err != nil {
				err = errors.Wrap(err, "You need to add learning data first")
				return
			}
			defer d.Close()
			var efficacy Efficacy

			minutesAgoInt := 60
			millisecondsAgo := int64(minutesAgoInt * 60 * 1000)
			sensors, err := d.GetSensorFromGreaterTime(millisecondsAgo)
			logger.Log.Debugf("[%s] got sensor from greater time %s", family, time.Since(startTime))
			devicesToCheckMap := make(map[string]struct{})
			for _, sensor := range sensors {
				devicesToCheckMap[sensor.Device] = struct{}{}
			}
			// get list of devices I care about
			devicesToCheck := make([]string, len(devicesToCheckMap))
			i := 0
			for device := range devicesToCheckMap {
				devicesToCheck[i] = device
				i++
			}
			logger.Log.Debugf("[%s] found %d devices to check", family, len(devicesToCheck))

			// More code here...

			return
		}(family)
		if err != nil {
			logger.Log.Warn(err)
			c.HTML(200, "dashboard.tmpl", gin.H{
				"ErrorMessage": err.Error(),
				"Family":       family,
			})
		}
	})
	return r.Run(":" + Port)
}
