package function

import (
	"encoding/json"
	"os"
	"testing"
)

func TestGetEvent_ReadSecrets(t *testing.T) {

	valSt := []string{"s1", "s2"}
	val, _ := json.Marshal(valSt)
	os.Setenv("Http_Secrets", string(val))
	owner := "alexellis"
	os.Setenv("Http_Owner", owner)
	installation_id := "123456"
	os.Setenv("Http_Installation_id", installation_id)
	eventInfo, err := getEvent()
	if err != nil {
		t.Errorf(err.Error())
		t.Fail()
	}

	expected := []string{owner + "-s1", owner + "-s2"}
	for _, val := range eventInfo.Secrets {
		found := false
		for _, expectedVal := range expected {
			if expectedVal == val {
				found = true
			}
		}
		if !found {
			t.Errorf("Wanted secret: %s, didn't find it in list", val)
		}
	}
}

func TestGetEvent_EmptyEnvVars(t *testing.T) {
	_, err := getEvent()

	if err != nil {
		t.Errorf(err.Error())
		t.Fail()
	}
}

func Test_GetImageName(t *testing.T) {

	var imageNameTestcases = []struct {
		Name              string
		PushRepositoryURL string
		RepositoryURL     string
		ImageName         string
		Output            string
	}{
		{
			"Test Docker Hub with user-prefix",
			"docker.io/of-community/",
			"docker.io/of-community/",
			"docker.io/of-community/function-name/",
			"docker.io/of-community/function-name/",
		},
		{
			"Testcase1",
			"registry:5000",
			"127.0.0.1:5000",
			"registry:5000/username/function-name/",
			"127.0.0.1:5000/username/function-name/",
		},
		{
			"Testcase2",
			"registry:31115",
			"127.0.0.1:31115",
			"registry:31115/username/function-name/",
			"127.0.0.1:31115/username/function-name/",
		},
		{
			"Testcase3",
			"registry:31115",
			"127.0.0.1",
			"registry:31115/username/function-name/",
			"127.0.0.1/username/function-name/",
		},
	}

	for _, testcase := range imageNameTestcases {
		t.Run(testcase.Name, func(t *testing.T) {
			output := getImageName(testcase.RepositoryURL, testcase.PushRepositoryURL, testcase.ImageName)
			if output != testcase.Output {
				t.Errorf("%s failed!. got: %s, want: %s", testcase.Name, output, testcase.Output)
			}
		})
	}
}

func Test_ValidImage(t *testing.T) {
	imageNames := map[string]bool{
		"failed to solve: rpc error: code = Unknown desc = exit code 2":   false,
		"failed to solve: rpc error: code = Unknown desc = exit status 2": false,
		"failed to solve:":                                                false,
		"error:":                                                          false,
		"code =":                                                          false,
		"127.0.0.1:5000/someuser/regex_go-regex_py:latest":                                                      true,
		"registry:5000/someuser/regex_go-regex_py:latest-7f7ec13d12b1397408e54b79686d43e41974bfa0":              true,
		"127.0.0.1:5000/someuser/regex_go-regex_py:latest-7f7ec13d12b1397408e54b79686d43e41974bfa0":             true,
		"docker.io/ofcommunity/someuser/regex_go-regex_py:latest-7f7ec13d12b1397408e54b79686d43e41974bfa0":      true,
		"docker.io:8080/ofcommunity/someuser/regex_go-regex_py:latest-7f7ec13d12b1397408e54b79686d43e41974bfa0": true,
	}
	for image, expected := range imageNames {
		if validImage(image) != expected {
			t.Errorf("For image %s, got: %v, want: %v", image, !expected, expected)
		}
	}
}

func Test_getReadOnlyRootFS_default(t *testing.T) {
	os.Setenv("readonly_root_filesystem", "1")

	val := getReadOnlyRootFS()
	want := true
	if val != want {
		t.Errorf("want %t, but got %t", want, val)
		t.Fail()
	}
}

func Test_getReadOnlyRootFS_override(t *testing.T) {
	os.Setenv("readonly_root_filesystem", "false")

	val := getReadOnlyRootFS()
	want := false
	if val != want {
		t.Errorf("want %t, but got %t", want, val)
		t.Fail()
	}
}
