package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("tasks")
		if err != nil {
			return err
		}

		// Set rules to public for distributed agents
		collection.ListRule = types.Pointer("")
		collection.ViewRule = types.Pointer("")
		collection.UpdateRule = types.Pointer("")
		collection.CreateRule = types.Pointer("")

		return app.Save(collection)
	}, func(app core.App) error {
		// Revert to admin-only? Or leave as is.
		return nil
	})
}
