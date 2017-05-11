package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/bakins/davy"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "davy IN OUTPUT",
	Short: "simple template generator",
	Run:   runSingleDir,
}

var (
	envDir     *string
	helperDir  *string
	clusterDir *string
	recurse    *bool
)

func main() {
	envDir = rootCmd.PersistentFlags().StringP("envs", "", "./envs", "directory containing environment overlays")
	clusterDir = rootCmd.PersistentFlags().StringP("clusters", "", "./clusters", "directory containing cluster overlays")
	helperDir = rootCmd.PersistentFlags().StringP("helpers", "", "./helpers", "directory containing helper template files (*.tpl)")
	recurse = rootCmd.PersistentFlags().BoolP("recurse", "r", false, "recurse into directory assuming it is a directory of application directories")
	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}

func runSingleDir(cmd *cobra.Command, args []string) {
	if len(args) != 2 {
		fmt.Println("indir and outdir are required")
		os.Exit(-4)
	}

	d, err := davy.New(
		davy.SetClusterDir(*clusterDir),
		davy.SetEnvDir(*envDir),
		davy.SetOutDir(args[1]),
	)

	if err != nil {
		fmt.Printf("failed to create generator: %s", err)
	}

	if err = d.ReadHelpers(filepath.Join(*helperDir, "*.tpl")); err != nil {
		fmt.Printf("failed to readhelpers: %s", err)
	}

	if !*recurse {
		if err = d.ProcessDir(args[0]); err != nil {
			fmt.Printf("failed to process directory: %s", err)
		}
	}

	entries, err := ioutil.ReadDir(args[0])
	if err != nil {
		fmt.Printf("failed to read directory: %s", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			if err = d.ProcessDir(entry.Name()); err != nil {
				fmt.Printf("failed to process directory: %s %s", entry.Name(), err)
			}
		}
	}
}
