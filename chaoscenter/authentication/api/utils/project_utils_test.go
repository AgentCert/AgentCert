package utils

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/litmuschaos/litmus/chaoscenter/authentication/api/types"
	"github.com/litmuschaos/litmus/chaoscenter/authentication/pkg/entities"
	"go.mongodb.org/mongo-driver/bson"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newCtx builds a gin context with the given query params and optional uid.
func newCtx(query url.Values, uid string) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/?"+query.Encode(), nil)
	c.Request = req
	if uid != "" {
		c.Set("uid", uid)
	}
	return c
}

func TestGetProjectFilters_Defaults(t *testing.T) {
	c := newCtx(url.Values{}, "user-1")
	req := GetProjectFilters(c)

	if req.UserID != "user-1" {
		t.Errorf("UserID = %q, want user-1", req.UserID)
	}
	if req.Filter == nil || req.Sort == nil {
		t.Fatal("Filter and Sort should be initialized")
	}
	// Default sort field is time.
	if req.Sort.Field == nil || *req.Sort.Field != entities.ProjectSortingFieldTime {
		t.Errorf("default sort field = %v, want TIME", req.Sort.Field)
	}
	// Default pagination.
	if req.Pagination == nil || req.Pagination.Page != 0 || req.Pagination.Limit != 15 {
		t.Errorf("default pagination = %+v, want page 0 limit 15", req.Pagination)
	}
}

func TestGetProjectFilters_AllParams(t *testing.T) {
	q := url.Values{}
	q.Set(types.CreatedByMe, "true")
	q.Set(types.ProjectName, "my-proj")
	q.Set(types.SortField, "name")
	q.Set(types.Ascending, "true")
	q.Set(types.Page, "2")
	q.Set(types.Limit, "25")

	req := GetProjectFilters(newCtx(q, "user-2"))

	if req.Filter.CreatedByMe == nil || !*req.Filter.CreatedByMe {
		t.Error("expected CreatedByMe true")
	}
	if req.Filter.ProjectName == nil || *req.Filter.ProjectName != "my-proj" {
		t.Errorf("ProjectName = %v, want my-proj", req.Filter.ProjectName)
	}
	if *req.Sort.Field != entities.ProjectSortingFieldName {
		t.Errorf("sort field = %v, want NAME", *req.Sort.Field)
	}
	if req.Sort.Ascending == nil || !*req.Sort.Ascending {
		t.Error("expected ascending true")
	}
	if req.Pagination.Page != 2 || req.Pagination.Limit != 25 {
		t.Errorf("pagination = %+v, want page 2 limit 25", req.Pagination)
	}
}

func TestGetProjectFilters_TimeSort(t *testing.T) {
	q := url.Values{}
	q.Set(types.SortField, "time")
	req := GetProjectFilters(newCtx(q, "u"))
	if *req.Sort.Field != entities.ProjectSortingFieldTime {
		t.Errorf("sort field = %v, want TIME", *req.Sort.Field)
	}
}

func TestCreateMatchStage(t *testing.T) {
	stage := CreateMatchStage("user-7")
	if len(stage) != 1 {
		t.Fatalf("expected 1 top-level element, got %d", len(stage))
	}
	if stage[0].Key != "$match" {
		t.Errorf("expected $match stage, got %q", stage[0].Key)
	}
}

func TestCreateFilterStages(t *testing.T) {
	t.Run("nil filter yields no stages", func(t *testing.T) {
		if stages := CreateFilterStages(nil, "u"); stages != nil {
			t.Errorf("expected nil, got %v", stages)
		}
	})

	t.Run("created by me true", func(t *testing.T) {
		cm := true
		stages := CreateFilterStages(&entities.ListProjectInputFilter{CreatedByMe: &cm}, "u1")
		if len(stages) != 1 {
			t.Fatalf("expected 1 stage, got %d", len(stages))
		}
	})

	t.Run("created by me false plus name", func(t *testing.T) {
		cm := false
		name := "abc"
		stages := CreateFilterStages(&entities.ListProjectInputFilter{CreatedByMe: &cm, ProjectName: &name}, "u1")
		if len(stages) != 2 {
			t.Fatalf("expected 2 stages, got %d", len(stages))
		}
	})

	t.Run("empty filter yields no stages", func(t *testing.T) {
		stages := CreateFilterStages(&entities.ListProjectInputFilter{}, "u1")
		if len(stages) != 0 {
			t.Errorf("expected 0 stages, got %d", len(stages))
		}
	})
}

func TestCreateSortStage(t *testing.T) {
	name := entities.ProjectSortingFieldName
	tm := entities.ProjectSortingFieldTime
	asc := true
	desc := false

	t.Run("nil sort", func(t *testing.T) {
		if got := CreateSortStage(nil); len(got) != 0 {
			t.Errorf("expected empty stage, got %v", got)
		}
	})

	t.Run("nil field", func(t *testing.T) {
		if got := CreateSortStage(&entities.SortInput{}); len(got) != 0 {
			t.Errorf("expected empty stage, got %v", got)
		}
	})

	t.Run("name ascending", func(t *testing.T) {
		stage := CreateSortStage(&entities.SortInput{Field: &name, Ascending: &asc})
		if stage[0].Key != "$sort" {
			t.Fatalf("expected $sort, got %q", stage[0].Key)
		}
		inner := stage[0].Value.(bson.D)
		if inner[0].Key != "name" {
			t.Errorf("expected sort by name, got %q", inner[0].Key)
		}
		if inner[0].Value != 1 {
			t.Errorf("expected ascending direction 1, got %v", inner[0].Value)
		}
	})

	t.Run("time descending default direction", func(t *testing.T) {
		stage := CreateSortStage(&entities.SortInput{Field: &tm, Ascending: &desc})
		if stage[0].Key != "$sort" {
			t.Fatalf("expected $sort, got %q", stage[0].Key)
		}
		inner := stage[0].Value.(bson.D)
		if inner[0].Key != "updated_at" {
			t.Errorf("expected sort by updated_at, got %q", inner[0].Key)
		}
		if inner[0].Value != -1 {
			t.Errorf("expected descending direction -1, got %v", inner[0].Value)
		}
	})
}

func TestCreatePaginationStage(t *testing.T) {
	t.Run("nil pagination defaults to limit 10", func(t *testing.T) {
		stages := CreatePaginationStage(nil)
		if len(stages) != 1 {
			t.Fatalf("expected 1 stage, got %d", len(stages))
		}
		if stages[0][0].Key != "$limit" {
			t.Errorf("expected $limit, got %q", stages[0][0].Key)
		}
		if stages[0][0].Value != 10 {
			t.Errorf("expected default limit 10, got %v", stages[0][0].Value)
		}
	})

	t.Run("normal pagination produces skip and limit", func(t *testing.T) {
		stages := CreatePaginationStage(&entities.Pagination{Page: 2, Limit: 20})
		if len(stages) != 2 {
			t.Fatalf("expected 2 stages, got %d", len(stages))
		}
		if stages[0][0].Key != "$skip" || stages[0][0].Value != 40 {
			t.Errorf("skip stage = %+v, want skip 40", stages[0][0])
		}
		if stages[1][0].Key != "$limit" || stages[1][0].Value != 20 {
			t.Errorf("limit stage = %+v, want limit 20", stages[1][0])
		}
	})

	t.Run("limit capped at 50", func(t *testing.T) {
		stages := CreatePaginationStage(&entities.Pagination{Page: 1, Limit: 100})
		if stages[1][0].Value != 50 {
			t.Errorf("expected limit capped to 50, got %v", stages[1][0].Value)
		}
		// skip uses capped limit.
		if stages[0][0].Value != 50 {
			t.Errorf("expected skip 50 (page*cappedLimit), got %v", stages[0][0].Value)
		}
	})
}
