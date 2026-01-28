package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/pearcec/hal9000/discovery/lmc"
	"github.com/spf13/cobra"
)

var (
	libraryPath string
	jsonOutput  bool
)

var libraryCmd = &cobra.Command{
	Use:   "library",
	Short: "Access HAL's Logic Memory Center",
	Long: `The Logic Memory Center stores and retrieves knowledge entities.
"I know I've made some very poor decisions recently, but I can give you
my complete assurance that my work will be back to normal."

Entity IDs follow the format: type/name (e.g., people/dave-bowman)`,
}

var libraryReadCmd = &cobra.Command{
	Use:   "read <entity-id>",
	Short: "Read an entity from the library",
	Long: `Read an entity by its ID (format: type/name).

Example:
  hal9000 library read people/dave-bowman
  hal9000 library read calendar/2026-01-27_meeting123`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		lib, err := getLibrary()
		if err != nil {
			return err
		}

		entity, err := lib.Get(args[0])
		if err != nil {
			return fmt.Errorf("failed to read entity: %w", err)
		}

		return outputEntity(entity)
	},
}

var libraryWriteCmd = &cobra.Command{
	Use:   "write <entity-id>",
	Short: "Write an entity to the library",
	Long: `Write an entity to the library. Content is read from stdin as JSON.

Example:
  echo '{"name": "Dave Bowman", "role": "Mission Commander"}' | hal9000 library write people/dave-bowman

  # With links:
  echo '{"name": "Dave", "links": [{"to": "missions/discovery-one", "type": "assigned_to"}]}' | hal9000 library write people/dave`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		lib, err := getLibrary()
		if err != nil {
			return err
		}

		// Parse entity ID
		parts := strings.SplitN(args[0], "/", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid entity ID format, expected type/name: %s", args[0])
		}
		entityType, entityName := parts[0], parts[1]

		// Read content from stdin
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		var data map[string]interface{}
		if err := json.Unmarshal(input, &data); err != nil {
			return fmt.Errorf("invalid JSON input: %w", err)
		}

		// Extract links if present
		var links []lmc.Edge
		if linksRaw, ok := data["links"].([]interface{}); ok {
			for _, l := range linksRaw {
				if linkMap, ok := l.(map[string]interface{}); ok {
					links = append(links, lmc.Edge{
						To:    getString(linkMap, "to"),
						Type:  getString(linkMap, "type"),
						Label: getString(linkMap, "label"),
					})
				}
			}
			delete(data, "links")
		}

		entity, err := lib.Store(entityType, entityName, data, links)
		if err != nil {
			return fmt.Errorf("failed to store entity: %w", err)
		}

		if jsonOutput {
			return outputEntity(entity)
		}
		fmt.Printf("Stored entity: %s\n", entity.ID)
		return nil
	},
}

var (
	queryType     string
	queryContains string
	queryLimit    int
)

var libraryQueryCmd = &cobra.Command{
	Use:   "query",
	Short: "Search entities in the library",
	Long: `Search for entities matching the given criteria.

Examples:
  hal9000 library query --type=people
  hal9000 library query --type=calendar --contains="meeting"
  hal9000 library query --contains="HAL" --limit=5`,
	RunE: func(cmd *cobra.Command, args []string) error {
		lib, err := getLibrary()
		if err != nil {
			return err
		}

		opts := lmc.QueryOptions{
			Type:     queryType,
			Contains: queryContains,
			Limit:    queryLimit,
		}

		entities, err := lib.Query(opts)
		if err != nil {
			return fmt.Errorf("query failed: %w", err)
		}

		return outputEntities(entities)
	},
}

var libraryListCmd = &cobra.Command{
	Use:   "list [type]",
	Short: "List entities by type or list all types",
	Long: `List entities of a specific type, or list all available types if no argument given.

Examples:
  hal9000 library list           # List all entity types
  hal9000 library list people    # List all people entities
  hal9000 library list calendar  # List all calendar entities`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		lib, err := getLibrary()
		if err != nil {
			return err
		}

		if len(args) == 0 {
			// List types
			types, err := lib.ListTypes()
			if err != nil {
				return fmt.Errorf("failed to list types: %w", err)
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(types, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(types) == 0 {
				fmt.Println("No entity types found in library.")
				return nil
			}

			fmt.Println("Entity types:")
			for _, t := range types {
				fmt.Printf("  %s\n", t)
			}
			return nil
		}

		// List entities of type
		entities, err := lib.Query(lmc.QueryOptions{
			Type:  args[0],
			Limit: queryLimit,
		})
		if err != nil {
			return fmt.Errorf("query failed: %w", err)
		}

		return outputEntities(entities)
	},
}

func init() {
	// Global library flags
	libraryCmd.PersistentFlags().StringVar(&libraryPath, "library-path", "", "Override default library location")
	libraryCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	// Query flags
	libraryQueryCmd.Flags().StringVar(&queryType, "type", "", "Filter by entity type")
	libraryQueryCmd.Flags().StringVar(&queryContains, "contains", "", "Filter by content text")
	libraryQueryCmd.Flags().IntVar(&queryLimit, "limit", 0, "Maximum number of results")

	// List flags
	libraryListCmd.Flags().IntVar(&queryLimit, "limit", 0, "Maximum number of results")

	// Add subcommands
	libraryCmd.AddCommand(libraryReadCmd)
	libraryCmd.AddCommand(libraryWriteCmd)
	libraryCmd.AddCommand(libraryQueryCmd)
	libraryCmd.AddCommand(libraryListCmd)
}

func getLibrary() (*lmc.Library, error) {
	// Suppress lmc package logging for CLI
	log.SetOutput(io.Discard)

	path := libraryPath
	if path == "" {
		path = "~/Documents/Google Drive/Claude/"
	}
	return lmc.New(path)
}

func outputEntity(entity *lmc.Entity) error {
	if jsonOutput {
		data, err := json.MarshalIndent(entity, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("ID:       %s\n", entity.ID)
	fmt.Printf("Type:     %s\n", entity.Type)
	fmt.Printf("Modified: %s\n", entity.Modified.Format("2006-01-02 15:04:05"))

	if len(entity.Content) > 0 {
		fmt.Println("Content:")
		content, _ := json.MarshalIndent(entity.Content, "  ", "  ")
		fmt.Printf("  %s\n", content)
	}

	if len(entity.Links) > 0 {
		fmt.Println("Links:")
		for _, link := range entity.Links {
			if link.Label != "" {
				fmt.Printf("  -> %s (%s: %s)\n", link.To, link.Type, link.Label)
			} else {
				fmt.Printf("  -> %s (%s)\n", link.To, link.Type)
			}
		}
	}

	return nil
}

func outputEntities(entities []*lmc.Entity) error {
	if jsonOutput {
		data, err := json.MarshalIndent(entities, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	if len(entities) == 0 {
		fmt.Println("No entities found.")
		return nil
	}

	fmt.Printf("Found %d entities:\n", len(entities))
	for _, e := range entities {
		fmt.Printf("  %s  (%s)\n", e.ID, e.Modified.Format("2006-01-02"))
	}
	return nil
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
