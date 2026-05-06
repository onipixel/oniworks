package database

import "reflect"

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

	switch hook {
	case hookBeforeCreate:
		if h, ok := v.(beforeCreate); ok {
			return h.BeforeCreate(db)
		}
	case hookAfterCreate:
		if h, ok := v.(afterCreate); ok {
			return h.AfterCreate(db)
		}
	case hookBeforeSave:
		if h, ok := v.(beforeSave); ok {
			return h.BeforeSave(db)
		}
	case hookAfterSave:
		if h, ok := v.(afterSave); ok {
			return h.AfterSave(db)
		}
	case hookBeforeUpdate:
		if h, ok := v.(beforeUpdate); ok {
			return h.BeforeUpdate(db)
		}
	case hookAfterUpdate:
		if h, ok := v.(afterUpdate); ok {
			return h.AfterUpdate(db)
		}
	case hookBeforeDelete:
		if h, ok := v.(beforeDelete); ok {
			return h.BeforeDelete(db)
		}
	case hookAfterDelete:
		if h, ok := v.(afterDelete); ok {
			return h.AfterDelete(db)
		}
	case hookAfterFind:
		if h, ok := v.(afterFind); ok {
			return h.AfterFind(db)
		}
	}
	return nil
}

// callSliceHook calls AfterFind on every element of a slice pointer.
func callSliceHook(slicePtr any, hook string, db *DB) {
	if hook != hookAfterFind {
		return
	}
	type afterFind interface{ AfterFind(*DB) error }
	rv := reflect.ValueOf(slicePtr)
	for rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Slice {
		return
	}
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i)
		// Get addressable value
		var iface any
		if elem.Kind() == reflect.Ptr {
			iface = elem.Interface()
		} else if elem.CanAddr() {
			iface = elem.Addr().Interface()
		} else {
			continue
		}
		if h, ok := iface.(afterFind); ok {
			_ = h.AfterFind(db)
		}
	}
}
