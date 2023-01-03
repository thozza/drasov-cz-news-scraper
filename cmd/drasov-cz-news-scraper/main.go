/*
 * www.drasov.cz/uredni-deska news scraper
 *
 * Copyright (C) 2023  Tomáš Hozza
 */

package main

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"golang.org/x/exp/maps"
)

type NewsEntryAttachment struct {
	Filename string
	URL      string
}

type NewsEntry struct {
	PublishedOn    *time.Time
	PublishedUntil *time.Time
	Title          string
	EntryURL       string
	Attachments    []NewsEntryAttachment
}

type News []*NewsEntry

// Since returns all news entries that were published since the given time, including the given time.
func (n News) SinceIncluding(t time.Time) News {
	var news News
	for _, newsEntry := range n {
		if newsEntry.PublishedOn.After(t) || newsEntry.PublishedOn.Equal(t) {
			news = append(news, newsEntry)
		}
	}
	return news
}

// NowDate returns the current date without the clock time, ignoring the timezone.
func NowDate() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

// StringDateToTime converts a string date in the format "DD. MM. YYYY" to a time.Time object.
func StringDateToTime(date string) (*time.Time, error) {
	// expected format: "1. 12. 2021"
	parts := strings.Split(date, ".")

	if len(parts) != 3 {
		return nil, fmt.Errorf("unexpected date format: %s", date)
	}

	day, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, err
	}

	month, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return nil, err
	}

	year, err := strconv.Atoi(strings.TrimSpace(parts[2]))
	if err != nil {
		return nil, err
	}

	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return &t, nil
}

// ScrapeNewsEntries scrapes all news entries from the www.drasov.cz/uredni-deska website.
func ScrapeNewsEntries() (News, error) {
	// map of news entries by their URL
	news := map[string]*NewsEntry{}

	allowedDomains := colly.AllowedDomains("drasov.cz", "www.drasov.cz")

	detailsCollector := colly.NewCollector(allowedDomains)

	detailsCollector.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL)
	})

	detailsCollector.OnHTML(".c-card", func(e *colly.HTMLElement) {
		newsEntry, ok := news[e.Request.URL.String()]
		if !ok {
			panic(fmt.Sprintf("news entry not found for URL %s", e.Request.URL))
		}

		// extract attachments
		e.ForEach(".c-files-wrapper", func(_ int, e *colly.HTMLElement) {
			newsEntry.Attachments = append(newsEntry.Attachments, NewsEntryAttachment{
				Filename: e.ChildText("h3"),
				URL:      e.ChildAttr("a", "href"),
			})
		})
	})

	allEntriesCollector := colly.NewCollector(allowedDomains)

	allEntriesCollector.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL)
	})

	allEntriesCollector.OnHTML(".c-office-board", func(e *colly.HTMLElement) {
		// iterate over all news entries
		e.ForEach(".c-office-board__content-item", func(_ int, e *colly.HTMLElement) {
			newsEntry := NewsEntry{}

			// extract PublishedOn and PublishedUntil dates
			e.ForEach(".c-office-board__col-date", func(idx int, e *colly.HTMLElement) {
				date, err := StringDateToTime(e.ChildTexts("span")[1])
				if err != nil {
					panic(fmt.Sprintf("error while parsing date: %s", err))
				}

				if idx == 0 {
					newsEntry.PublishedOn = date
				} else if idx == 1 {
					newsEntry.PublishedUntil = date
				} else {
					panic("unexpected index while iterating over .c-office-board__col-date")
				}
			})

			// extract Title and EntryURL
			e.ForEachWithBreak(".c-office-board__col-name-content", func(_ int, e *colly.HTMLElement) bool {
				newsEntry.Title = e.ChildText("a")
				newsEntry.EntryURL = fmt.Sprintf("https://www.drasov.cz%s", e.ChildAttr("a", "href"))
				return false
			})

			news[newsEntry.EntryURL] = &newsEntry
			err := detailsCollector.Visit(newsEntry.EntryURL)
			if err != nil {
				panic(fmt.Sprintf("error while collecting details from %s: %s", newsEntry.EntryURL, err))
			}
		})
	})

	err := allEntriesCollector.Visit("https://www.drasov.cz/uredni-deska")
	if err != nil {
		return nil, err
	}

	allEntriesCollector.Wait()
	detailsCollector.Wait()

	return maps.Values(news), nil
}

func main() {
	minusDays := flag.Int("days", 30, "filter news entries published in the last N days")
	flag.Parse()

	sinceDate := NowDate().AddDate(0, 0, -*minusDays)

	news, err := ScrapeNewsEntries()
	if err != nil {
		panic(err)
	}

	for _, newsEntry := range news.SinceIncluding(sinceDate) {
		fmt.Printf("%+v\n", newsEntry)
	}
}
