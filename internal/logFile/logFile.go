package logFile

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Jeselnik/rb-scrobbler/internal/track"
)

const (
	AUDIOSCROBBLER_HEADER = "#AUDIOSCROBBLER/"
	SEPARATOR             = "\t"
	NEWLINE               = "\n"
	LISTENED              = "L"
	ARTIST_INDEX          = 0
	ALBUM_INDEX           = 1
	TITLE_INDEX           = 2
	RATING_INDEX          = 5
	TIMESTAMP_INDEX       = 6
	TIMESTAMP_NO_RTC      = "0"
)

var ErrTrackSkipped = errors.New("track was skipped")
var ErrInvalidLog = errors.New("invalid .scrobbler.log")

/* Take a path to a file and return a representation of that file in a
string slice where every line is a value */
func ImportLog(path *string) ([]string, error) {
	logFile, err := os.Open(*path)
	if err != nil {
		return []string{}, err
	}

	/* It's important to not log.Fatal() or os.Exit() within this function
	as the following statement will not execute and "let go" of the given
	file */
	defer logFile.Close()

	logInBytes, err := ioutil.ReadAll(logFile)
	if err != nil {
		return []string{}, err
	}

	logAsLines := strings.Split(string(logInBytes), NEWLINE)
	/* Ensure that the file is actually an audioscrobbler log */
	if !strings.Contains(logAsLines[0], AUDIOSCROBBLER_HEADER) {
		return []string{}, ErrInvalidLog
	} else {
		return logAsLines, nil
	}
}

/* Take a string, split it, convert time if needed and return a track */
func LineToTrack(line, offset string) (track.Track, error) {
	splitLine := strings.Split(line, SEPARATOR)

	/* Check the "RATING" index instead of looking for "\tL\t" in a line,
	just in case a track or album is named "L". If anything like this exists
	and was skipped the old method would false positive it as listened
	and then it'd be submitted */
	if splitLine[RATING_INDEX] == LISTENED {
		var timestamp string = splitLine[TIMESTAMP_INDEX]

		/* If user has a player with no Real Time Clock, the log file gives it
		a timestamp of 0. Last.fm API doesn't accept scrobbles dated that far
		into the past so in the interests of at least having the tracks sent,
		date them with current local time */
		if timestamp == TIMESTAMP_NO_RTC {
			timestamp = strconv.FormatInt(time.Now().Unix(), 10)
		}

		/* Time conversion - the API wants it in UTC timezone */
		if offset != "0h" {
			timestamp = convertTimeStamp(timestamp, offset)
		}

		track := track.Track{
			Artist:    splitLine[ARTIST_INDEX],
			Album:     splitLine[ALBUM_INDEX],
			Title:     splitLine[TITLE_INDEX],
			Timestamp: timestamp,
		}

		return track, nil

	} else {
		return track.Track{}, ErrTrackSkipped
	}
}

/* Convert back/to UTC from localtime */
func convertTimeStamp(timestamp, offset string) string {
	/* Log stores it in unix epoch format. Convert to an int
	so it can be manipulated with the time package */
	timestampInt, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	/* Convert the epoch into a Time type for conversion.
	The 0 stands for 0 milliseconds since epoch which isn't
	needed */
	trackTime := time.Unix(timestampInt, 0)

	/* Take the offset flag and convert it into the duration
	which will be added/subtracted */
	newOffset, err := time.ParseDuration(offset)
	if err != nil {
		log.Fatal(err)
	}

	/* The duration is negative so that entries behind UTC are
	brought forward while entries ahead are brought back */
	convertedTime := trackTime.Add(-newOffset)
	return strconv.FormatInt(convertedTime.Unix(), 10)
}

func deleteLogFile(path *string) (exitCode int) {
	deletionError := os.Remove(*path)
	if deletionError != nil {
		fmt.Printf("Error Deleting %q!\n%v\n", *path, deletionError)
		exitCode = 1
	} else {
		fmt.Printf("%q Deleted!\n", *path)
	}
	return exitCode
}

func HandleFile(nonInteractive, logPath *string, fail uint) int {
	exitCode := 0

	switch *nonInteractive {
	case "keep":
		fmt.Printf("%q kept\n", *logPath)

	case "delete":
		exitCode = deleteLogFile(logPath)

	case "delete-on-success":
		if fail == 0 {
			exitCode = deleteLogFile(logPath)
		} else {
			fmt.Printf("Scrobble failures: %q not deleted.\n", *logPath)
			exitCode = 1
		}

	default:
		reader := bufio.NewReader(os.Stdin)
		var input string
		fmt.Printf("Delete %q? [y/n] ", *logPath)
		input, err := reader.ReadString('\n')
		fmt.Print("\n")
		if err != nil {
			fmt.Printf("Error reading input! File %q not deleted.\n%v\n",
				*logPath, err)
			exitCode = 1
		} else if strings.ContainsAny(input, "y") ||
			strings.ContainsAny(input, "Y") {
			deleteLogFile(logPath)
		} else {
			fmt.Printf("%q kept.\n", *logPath)
		}
	}

	return exitCode
}
