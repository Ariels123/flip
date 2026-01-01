package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	// Migration to add 'mode' and 'last_seen' to 'agents' collection
	// mode: "local" (daemon executes) or "remote" (agent executes via listen)
	// last_seen: datetime of last heartbeat
	
	core.AppMigrations.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("agents")
		if err != nil {
			return err
		}


		// Add 'mode' field
		modeField := &core.SelectField{Name: "mode"}
		modeField.MaxSelect = 1
		modeField.Values = []string{"local", "remote"}
		modeField.Required = true
		collection.Fields.Add(modeField)

		// Add 'last_seen' field
		lastSeenField := &core.DateField{Name: "last_seen"}
		collection.Fields.Add(lastSeenField)

		return app.Save(collection)
	}, func(app core.App) error {
		// Revert
		collection, err := app.FindCollectionByNameOrId("agents")
		if err != nil {
			return err
		}
		
		collection.Fields.RemoveByName("mode")
		collection.Fields.RemoveByName("last_seen")
		
		return app.Save(collection)
	})
}
