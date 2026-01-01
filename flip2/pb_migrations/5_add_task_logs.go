package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("tasks")
		if err != nil {
			return err
		}

		// Add stdout_log field
		collection.Fields.Add(&core.TextField{
			Name:     "stdout_log",
			Required: false,
		})

		// Add stderr_log field
		collection.Fields.Add(&core.TextField{
			Name:     "stderr_log",
			Required: false,
		})

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("tasks")
		if err != nil {
			return err
		}

		// Remove stdout_log field
		if f := collection.Fields.GetByName("stdout_log"); f != nil {
			collection.Fields.RemoveById(f.GetId())
		}

		// Remove stderr_log field
		if f := collection.Fields.GetByName("stderr_log"); f != nil {
			collection.Fields.RemoveById(f.GetId())
		}

		return app.Save(collection)
	})
}
