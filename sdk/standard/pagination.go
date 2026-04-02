package standard

import (
	"context"
	"net/http"

	"github.com/NVIDIA/ncx-infra-controller-rest/sdk/standard/helpers"
)

const PaginationHeader = helpers.PaginationHeader

// PaginationResponse is the response contained in the x-pagination header of http response.
type PaginationResponse = helpers.PaginationResponse

// GetPaginationResponse extracts the pagination response from the JSON contained in the x-pagination header.
func GetPaginationResponse(ctx context.Context, response *http.Response) (*PaginationResponse, error) {
	return helpers.GetPaginationResponse(ctx, response)
}
