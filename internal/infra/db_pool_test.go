package infra

import "testing"

func TestPoolConfigRouting(t *testing.T) {
	t.Setenv("DROVER_PG_POOLER", "")
	cases := []struct {
		url  string
		port uint16
		max  int32
	}{
		{"postgres://u:p@abc.aws-ap-southeast-2.horizon.psdb.cloud:5432/registry?sslmode=require", 6432, 3},
		{"postgres://u:p@localhost:5432/registry?sslmode=disable", 5432, 3},
		{"postgres://u:p@localhost:5432/registry?sslmode=disable&pool_max_conns=10", 5432, 10},
	}
	for _, c := range cases {
		cfg, err := PoolConfig(c.url)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.ConnConfig.Port != c.port || cfg.MaxConns != c.max {
			t.Errorf("%s: port %d max %d, want %d and %d", c.url, cfg.ConnConfig.Port, cfg.MaxConns, c.port, c.max)
		}
	}
}
