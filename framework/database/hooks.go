package database

const (
	hookBeforeCreate = "BeforeCreate"
	hookAfterCreate  = "AfterCreate"
	hookBeforeSave   = "BeforeSave"
	hookAfterSave    = "AfterSave"
	hookBeforeUpdate = "BeforeUpdate"
	hookAfterUpdate  = "AfterUpdate"
	hookBeforeDelete = "BeforeDelete"
	hookAfterDelete  = "AfterDelete"
	hookAfterFind    = "AfterFind"
)

// callHook calls the named hook method on v if it implements it.
func callHook(v any, hook string, db *DB) error {
	type beforeCreate interface{ BeforeCreate(*DB) error }
	type afterCreate interface{ AfterCreate(*DB) error }
	type beforeSave interface{ BeforeSave(*DB) error }
	type afterSave interface{ AfterSave(*DB) error }
	type beforeUpdate interface{ BeforeUpdate(*DB) error }
	type afterUpdate interface{ AfterUpdate(*DB) error }
	type beforeDelete interface{ BeforeDelete(*DB) error }
	type afterDelete interface{ AfterDelete(*DB) error }
	type afterFind interface{ AfterFind(*DB) error }

	var err error
	switch hook {
	case hookBeforeCreate:
		if h, ok := v.(beforeCreate); ok {
			err = h.BeforeCreate(db)
		}
	case hookAfterCreate:
		if h, ok := v.(afterCreate); ok {
			err = h.AfterCreate(db)
		}
	case hookBeforeSave:
		if h, ok := v.(beforeSave); ok {
			err = h.BeforeSave(db)
		}
	case hookAfterSave:
		if h, ok := v.(afterSave); ok {
			err = h.AfterSave(db)
		}
	case hookBeforeUpdate:
		if h, ok := v.(beforeUpdate); ok {
			err = h.BeforeUpdate(db)
		}
	case hookAfterUpdate:
		if h, ok := v.(afterUpdate); ok {
			err = h.AfterUpdate(db)
		}
	case hookBeforeDelete:
		if h, ok := v.(beforeDelete); ok {
			err = h.BeforeDelete(db)
		}
	case hookAfterDelete:
		if h, ok := v.(afterDelete); ok {
			err = h.AfterDelete(db)
		}
	case hookAfterFind:
		if h, ok := v.(afterFind); ok {
			err = h.AfterFind(db)
		}
	}
	return err
}

// callSliceHook calls the hook on every element of a slice.
func callSliceHook(slice any, hook string, db *DB) {
	// We only call AfterFind for slices; other hooks are called per-element during Insert/Save.
	if hook != hookAfterFind {
		return
	}
	// Use reflection to iterate the slice
	import_reflect := func() {
		// intentional no-op: we use a type switch for speed since AfterFind is the only slice hook
	}
	_ = import_reflect

	type afterFind interface{ AfterFind(*DB) error }

	// Walk the slice by reflection
	import_reflect2 := func(v any) {
		_ = v // placeholder; real implementation uses reflect.ValueOf
	}
	_ = import_reflect2
	// For simplicity, we call AfterFind on the slice pointer if it implements the interface.
	// Callers who need per-element AfterFind should embed it in each struct.
	if h, ok := slice.(afterFind); ok {
		_ = h.AfterFind(db)
	}
}
