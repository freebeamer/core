package utils

func IntField(values map[string]string, key string) (int, error) {
	value, err := Integer(values, key, 0)
	return int(value), err
}
