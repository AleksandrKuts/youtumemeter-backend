package server

import (
	"encoding/json"
	"errors"
	"github.com/AleksandrKuts/youtubemeter-service/backend/config"
	"github.com/hashicorp/golang-lru"
	"time"
)

// Кеш для відео
var cacheVideo *lru.TwoQueueCache

var MIN_TIME = time.Time{}

func init() {
	var err error

	if *config.EnableCache {
		cacheVideo, err = lru.New2Q(*config.MaxSizeCacheVideo)
		if err != nil {
			log.Fatalf("err: %v", err)
		}
	}
}

// Отримати опис відео по його id
func getVideoById(id string) ([]byte, error) {
	if id == "" {
		return nil, errors.New("video id is null")
	}
	log.Debugf("id: %v", id)

	var ok bool = false
	var videoi interface{}
	
	if *config.EnableCache {
		videoi, ok = cacheVideo.Get(id)
		log.Debugf("id: %v, cache, have data?: %v", id, ok)

		// Якщо дані в кеші є, то беремо їх тільки якщо з останнього запиту пройшло часу менш
		// ніж період збору метрик, або якщо збір метрик вже припинився.
		if ok {
			video := videoi.(*VideoInCache)
			log.Debugf("id: %v, metrics: %v, video: %v, published: %v", id, video.updateMetrics, video.updateVideo, video.publishedAt)

			if time.Since(video.publishedAt) > *config.PeriodCollectionCache ||
				time.Since(video.updateVideo) < *config.PeriodMeterCache {

				log.Debugf("id: %v, cache, video %v", id, string(video.videoResponce))

				return video.videoResponce, nil
			}
		}
	}

	// В кеші актуальної інформации не знайдено, запрошуемо в БД
	youtubeVideo, err := getVideoByIdFromDB(id)
	if err != nil {
		return nil, err
	}

	stringJsonVideo, err := json.Marshal(*youtubeVideo)
	if err != nil {
		log.Errorf("Error convert select to video: response=%v, error=%v", *youtubeVideo, err)
		return nil, err
	}
	log.Debugf("id: %v, video=%v", id, string(stringJsonVideo))

	if *config.EnableCache {
		// Якщо дані по запиту вже в кеші, то тільки корегуємо їх
		if ok {
			// Корегуємо дані в кеші
			videoi.(*VideoInCache).updateCacheVideo(youtubeVideo.PublishedAt, stringJsonVideo)
			log.Debugf("id: %v, cache, update video, published: %v", id, youtubeVideo.PublishedAt)
		} else {
			// Додаємо запит до кешу
			cacheVideo.Add(id, &VideoInCache{MIN_TIME, time.Now(), youtubeVideo.PublishedAt, stringJsonVideo, nil})
			log.Debugf("id: %v, cache, add video, published: %v", id, youtubeVideo.PublishedAt)
		}
	}

	return stringJsonVideo, nil
}

// Отримати метрики по відео id за заданий період
func getMetricsByIdFromTo(id string, from, to string) ([]byte, error) {
	// В кеші актуальної інформации не знайдено, запрошуемо в БД
	response, err := getMetricsByIdFromDB(id, from, to)
	if err != nil {
		return nil, err
	}

	metricsVideoJson, err := json.Marshal(response)
	if err != nil {
		log.Errorf("Error convert select to Metrics: response=%v, error=%v", response, err)
		return nil, err
	}
	log.Debugf("id: %v, metrics=%v", id, string(metricsVideoJson))

	return metricsVideoJson, nil
}

// Отримати метрики по відео id за заданий період
func getMetricsById(id string, from, to string) ([]byte, error) {
	if id == "" {
		return nil, errors.New("video id is null")
	}
	log.Debugf("id: %v, from: %v, to: %v", id, from, to)

	// Перевірка чи є дані в кеші. В кеші зберігаються тільки запроси за весь період
	if from != "" || to != "" {
		return getMetricsByIdFromTo(id, from, to)
	}

	var ok bool = false
	var videoi interface{}
	
	if *config.EnableCache {
		videoi, ok = cacheVideo.Get(id)
		log.Debugf("id: %v, cache, have data? %v", id, ok)

		// Якщо дані в кеші є, то беремо їх тільки якщо з останнього запиту пройшло часу менш
		// ніж період збору метрик, або якщо збір метрик вже припинився.
		if ok {
			video := videoi.(*VideoInCache)
			log.Debugf("id: %v, metrics: %v, video: %v, published: %v", id, video.updateMetrics, video.updateVideo, video.publishedAt)

			if time.Since(video.updateMetrics) < *config.PeriodMeterCache ||
				time.Since(video.publishedAt) > *config.PeriodCollectionCache {

				log.Debugf("id: %v, cache, metrics: %v", id, string(video.metricsResponce))

				return video.metricsResponce, nil
			}
			log.Debug("cache, skip")
		}

	}

	// В кеші актуальної інформации не знайдено, запрошуемо в БД
	response, err := getMetricsByIdFromDB(id, from, to)
	if err != nil {
		return nil, err
	}

	metricsVideoJson, err := json.Marshal(response)
	if err != nil {
		log.Errorf("Error convert select to Metrics: response=%v, error=%v", response, err)
		return nil, err
	}
	log.Debugf("id: %v, metrics=%v", id, string(metricsVideoJson))

	if *config.EnableCache {
		// Якщо дані по запиту вже в кеші, то тільки корегуємо їх
		if ok {
			// Корегуємо дані в кеші
			videoi.(*VideoInCache).updateCacheMetrics(metricsVideoJson)
			log.Debugf("id: %v, cache, update metrics, published", id)
		} else {
			// Додаємо запит до кешу
			cacheVideo.Add(id, &VideoInCache{time.Now(), MIN_TIME, time.Now(), nil, metricsVideoJson})
			log.Debugf("id: %v, cache, add metrics", id)
		}
	}

	return metricsVideoJson, nil
}
