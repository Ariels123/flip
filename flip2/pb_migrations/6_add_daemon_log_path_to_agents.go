package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("agents")
		if err != nil {
			return err
		}

		// Add new text field for daemon_log_path
		collection.Fields.Add(&core.TextField{
			Name:     "daemon_log_path",
			Required: false,
		})

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("agents")
		if err != nil {
			return err
		}

		// Remove the field on downgrade
		if f := collection.Fields.GetByName("daemon_log_path"); f != nil {
			collection.Fields.RemoveById(f.GetId())
		}

		return app.Save(collection)
	})
}
