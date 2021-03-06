package cmd

import (
	"fmt"
	nurl "net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-shiori/shiori/internal/model"
	"github.com/spf13/cobra"
)

func pocketCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pocket source-file",
		Short: "Import bookmarks from Pocket's exported HTML file",
		Args:  cobra.ExactArgs(1),
		Run:   pocketHandler,
	}

	return cmd
}

func pocketHandler(cmd *cobra.Command, args []string) {
	// Prepare bookmark's ID
	bookID, err := db.CreateNewID("bookmark")
	if err != nil {
		cError.Printf("Failed to create ID: %v\n", err)
		return
	}

	// Open pocket's file
	srcFile, err := os.Open(args[0])
	if err != nil {
		cError.Println(err)
		return
	}
	defer srcFile.Close()

	// Parse pocket's file
	bookmarks := []model.Bookmark{}
	mapURL := make(map[string]struct{})

	doc, err := goquery.NewDocumentFromReader(srcFile)
	if err != nil {
		cError.Println(err)
		return
	}

	doc.Find("a").Each(func(_ int, a *goquery.Selection) {
		// Get metadata
		title := a.Text()
		url, _ := a.Attr("href")
		strTags, _ := a.Attr("tags")
		strModified, _ := a.Attr("time_added")
		intModified, _ := strconv.ParseInt(strModified, 10, 64)
		modified := time.Unix(intModified, 0)

		// Clean up URL by removing its fragment and UTM parameters
		tmp, err := nurl.Parse(url)
		if err != nil || tmp.Scheme == "" || tmp.Hostname() == "" {
			cError.Printf("Skip %s: URL is not valid\n", url)
			return
		}

		tmp.Fragment = ""
		clearUTMParams(tmp)
		url = tmp.String()

		// Make sure title is valid Utf-8
		title = toValidUtf8(title, url)

		// Check if the URL already exist before, both in bookmark
		// file or in database
		if _, exist := mapURL[url]; exist {
			cError.Printf("Skip %s: URL already exists\n", url)
			return
		}

		if _, exist := db.GetBookmark(0, url); exist {
			cError.Printf("Skip %s: URL already exists\n", url)
			mapURL[url] = struct{}{}
			return
		}

		// Get bookmark tags
		tags := []model.Tag{}
		for _, strTag := range strings.Split(strTags, ",") {
			if strTag != "" {
				tags = append(tags, model.Tag{Name: strTag})
			}
		}

		// Add item to list
		bookmark := model.Bookmark{
			ID:       bookID,
			URL:      url,
			Title:    normalizeSpace(title),
			Modified: modified.Format("2006-01-02 15:04:05"),
			Tags:     tags,
		}

		bookID++
		mapURL[url] = struct{}{}
		bookmarks = append(bookmarks, bookmark)
	})

	// Save bookmark to database
	bookmarks, err = db.SaveBookmarks(bookmarks...)
	if err != nil {
		cError.Printf("Failed to save bookmarks: %v\n", err)
		return
	}

	// Print imported bookmark
	fmt.Println()
	printBookmarks(bookmarks...)
}
