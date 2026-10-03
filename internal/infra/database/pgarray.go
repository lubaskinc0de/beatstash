package database

import (
	"context"
	"fmt"
	"reflect"

	"github.com/jackc/pgx/v5/pgtype"
	"gorm.io/gorm/schema"
)

func init() {
	schema.RegisterSerializer("pgarray", pgArray{})
}

// pgArray keeps a []string as a Postgres text[], a nil slice as NULL.
type pgArray struct{}

func (pgArray) Scan(ctx context.Context, field *schema.Field, dst reflect.Value, dbValue any) error {
	var values []string
	switch v := dbValue.(type) {
	case nil:
	case string:
		if err := scanTextArray([]byte(v), &values); err != nil {
			return err
		}
	case []byte:
		if err := scanTextArray(v, &values); err != nil {
			return err
		}
	default:
		return fmt.Errorf("pgarray: unexpected %T", dbValue)
	}
	return field.Set(ctx, dst, values)
}

func (pgArray) Value(_ context.Context, _ *schema.Field, _ reflect.Value, fieldValue any) (any, error) {
	values, _ := fieldValue.([]string)
	if values == nil {
		return nil, nil
	}
	return values, nil
}

func scanTextArray(src []byte, values *[]string) error {
	*values = []string{}
	return pgtype.NewMap().Scan(pgtype.TextArrayOID, pgtype.TextFormatCode, src, values)
}

// textArray spells the values as a text[] literal, for raw SQL: gorm would
// expand a slice into a list.
func textArray(values []string) (string, error) {
	literal, err := pgtype.NewMap().Encode(pgtype.TextArrayOID, pgtype.TextFormatCode, values, nil)
	return string(literal), err
}
