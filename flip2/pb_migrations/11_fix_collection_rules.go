package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {
		// Fix access rules for all collections to allow public read access
		collectionNames := []string{"agents", "signals", "costs", "alerts", "tasks", "code_reviews"}

		for _, name := range collectionNames {
			collection, err := app.FindCollectionByNameOrId(name)
			if err != nil {
				// Collection doesn't exist, skip
				continue
			}

			// Set public access rules (empty string = public access)
			collection.ListRule = types.Pointer("")
			collection.ViewRule = types.Pointer("")
			collection.CreateRule = types.Pointer("")
			collection.UpdateRule = types.Pointer("")
			collection.DeleteRule = types.Pointer("")

			if err := app.Save(collection); err != nil {
				return err
			}
		}

		return nil
	}, func(app core.App) error {
		// Rollback: Set rules back to null (no access)
		collectionNames := []string{"agents", "signals", "costs", "alerts", "tasks", "code_reviews"}

		for _, name := range collectionNames {
			collection, err := app.FindCollectionByNameOrId(name)
			if err != nil {
				continue
			}

			collection.ListRule = nil
			collection.ViewRule = nil
			collection.CreateRule = nil
			collection.UpdateRule = nil
			collection.DeleteRule = nil

			app.Save(collection)
		}

		return nil
	})
}
