/*
Copyright © 2022 Christian Kniep <christian@qnib.org>
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	mlver "github.com/metahub-registry/metahub-core/pkg/version"
)

var version = "2.6.2"
var jsonFlag bool

type Version struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
	Patch int `json:"patch"`
}

// parseCmd represents the parse command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Prints the version of the application",
	Run: func(cmd *cobra.Command, args []string) {
		if jsonFlag {
			vSplit := strings.Split(version, ".")
			major, _ := strconv.Atoi(vSplit[0])
			minor, _ := strconv.Atoi(vSplit[1])
			patch, _ := strconv.Atoi(vSplit[2])
			v := Version{
				Major: major,
				Minor: minor,
				Patch: patch,
			}
			b, err := json.Marshal(v) //, "", "\t")
			if err != nil {
				log.Fatal(err)
			}

			fmt.Println(string(b))
		} else {
			fmt.Printf("API:     v%s\n", version)
			fmt.Printf("Metalib: v%s\n", mlver.Version)
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	// Here you will define your flags and configuration settings.
	versionCmd.Flags().BoolVar(&jsonFlag, "json", false, "Prints the version in JSON format")
	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// parseCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// parseCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
