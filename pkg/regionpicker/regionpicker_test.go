package regionpicker

import (
	"testing"

	"github.com/sirupsen/logrus"
)

func TestFindBest(t *testing.T) {
	// should work in general
	regionName, err := FindBest(CloudGCP, nil, logrus.New())
	if err != nil {
		t.Errorf("got an error trying to find best region: Best(): %v", err)
		return
	}

	if regionName == "" {
		t.Errorf("got an empty region trying to find best region")
		return
	}

	// should work without a logger
	regionName, err = FindBest(CloudGCP, nil, nil)
	if err != nil {
		t.Errorf("got an error trying to find best region: Best(): %v", err)
		return
	}

	if regionName == "" {
		t.Errorf("got an empty region trying to find best region")
		return
	}

	// should support filtering
	regionName, err = FindBest(CloudGCP, []RegionName{"us"}, nil)
	if err != nil {
		t.Errorf("got an error trying to find best region: Best(): %v", err)
		return
	}

	if regionName != "us" {
		t.Errorf("got result other than filtered result: %v", regionName)
		return
	}

	// should return error when invalid sort
	_, err = FindBest(CloudGCP, []RegionName{"kjaefhjfhjkfbac"}, nil)
	if err == nil {
		t.Errorf("didn't get an error trying to find non-existent best region: Best(): %v", err)
		return
	}
}
