package run

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/kubesphere/s2irun/pkg/api"
	"github.com/kubesphere/s2irun/pkg/api/describe"
	"github.com/kubesphere/s2irun/pkg/api/validation"
	"github.com/kubesphere/s2irun/pkg/build/strategies"
	"github.com/kubesphere/s2irun/pkg/docker"
	s2ierr "github.com/kubesphere/s2irun/pkg/errors"
	"github.com/kubesphere/s2irun/pkg/outputresult"
	"github.com/kubesphere/s2irun/pkg/scm/git"
	"github.com/kubesphere/s2irun/pkg/utils/cmd"
	"github.com/kubesphere/s2irun/pkg/utils/fs"
	utilglog "github.com/kubesphere/s2irun/pkg/utils/glog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	ConfigEnvVariable = "S2I_CONFIG_PATH"
	KanikoEnvVariable = "KANIKO_EXEC_PATH"
)

var glog = utilglog.StderrLog

// S2I Just run the command
func S2I(cfg *api.Config) error {
	cfg.DockerConfig = docker.GetDefaultDockerConfig()
	if len(cfg.AsDockerfile) > 0 {
		if cfg.RunImage {
			return fmt.Errorf("ERROR: --run cannot be used with --as-dockerfile")
		}
		if len(cfg.RuntimeImage) > 0 {
			return fmt.Errorf("ERROR: --runtime-image cannot be used with --as-dockerfile")
		}
	}
	if cfg.Incremental && len(cfg.RuntimeImage) > 0 {
		return fmt.Errorf("ERROR: Incremental build with runtime image isn't supported")
	}
	//set default image pull policy
	if len(cfg.BuilderPullPolicy) == 0 {
		cfg.BuilderPullPolicy = api.DefaultBuilderPullPolicy
	}
	if len(cfg.PreviousImagePullPolicy) == 0 {
		cfg.PreviousImagePullPolicy = api.DefaultPreviousImagePullPolicy
	}
	if len(cfg.RuntimeImagePullPolicy) == 0 {
		cfg.RuntimeImagePullPolicy = api.DefaultRuntimeImagePullPolicy
	}
	if errs := validation.ValidateConfig(cfg); len(errs) > 0 {
		var buf bytes.Buffer
		for _, e := range errs {
			buf.WriteString("ERROR:")
			buf.WriteString(e.Error())
			buf.WriteString("\n")
		}
		return fmt.Errorf(buf.String())
	}

	client, err := docker.NewEngineAPIClient(cfg.DockerConfig)
	if err != nil {
		return err
	}

	d := docker.New(client, cfg.PullAuthentication, cfg.PushAuthentication)
	err = d.CheckReachable()
	if err != nil {
		return err
	}

	glog.V(9).Infof("\n%s\n", describe.Config(client, cfg))

	builder, _, err := strategies.GetStrategy(client, cfg)
	s2ierr.CheckError(err)
	result, err := builder.Build(cfg)
	if err != nil {
		glog.V(0).Infof("Build failed")
		s2ierr.CheckError(err)
		return err
	} else {
		if len(cfg.AsDockerfile) > 0 {
			glog.V(0).Infof("Application dockerfile generated in %s", cfg.AsDockerfile)
		} else {
			glog.V(0).Infof("Build completed successfully")
		}
	}

	//result.Message store Callback Info
	for _, message := range result.Messages {
		glog.V(1).Infof(message)
	}

	return nil
}

func App() int {
	var apiConfig = new(api.Config)
	path := os.Getenv(ConfigEnvVariable)
	file, err := os.Open(path)
	defer file.Close()
	if os.IsNotExist(err) {
		glog.Errorf("Config file does not exist,please check the path: %s", path)
		return 1
	}

	jsonParser := json.NewDecoder(file)
	err = jsonParser.Decode(apiConfig)
	if err != nil {
		glog.Errorf("There are some errors in config file, please check the error:\n%v", err)
		return 1
	}
	apiConfig.Source, err = git.Parse(apiConfig.SourceURL, apiConfig.IsBinaryURL)
	if err != nil {
		glog.Errorf("SourceURL is illegal, please check the error:\n%v", err)
		return 1
	}
	kanikoPath := os.Getenv(KanikoEnvVariable)
	// 配置了环境变量，则查看下文件存在不，如果文件不存在，则使用原来的逻辑
	if _, err := os.Stat(kanikoPath); err == nil {
		glog.Infof("kaniko path: %v", kanikoPath)
		originalName := apiConfig.Tag

		if git.HasGitBinary() {
			sgit := git.New(fs.NewFileSystem(), cmd.NewCommandRunner())
			os.MkdirAll(apiConfig.ContextDir, 0777)
			if err := sgit.Clone(apiConfig.Source, apiConfig.ContextDir, git.CloneConfig{Quiet: false}); err != nil {
				glog.Errorf("git clone failed %v", err)
				return 1
			}
			sourceInfo := sgit.GetInfo(apiConfig.ContextDir)
			commit := sourceInfo.CommitID
			if len(commit) > 8 {
				commit = commit[:8]
			}
			if !strings.Contains(originalName, "/") {
				glog.Infof("origin name:%s,has no repo, add username :%s", originalName, apiConfig.PushAuthentication.Username)
				originalName = apiConfig.PushAuthentication.Username + "/" + originalName
			}
			originalName = strings.ReplaceAll(originalName, "${DATE}", time.Now().Format("20060102150405"))
			originalName = strings.ReplaceAll(originalName, "${COMMIT}", commit)

			apiConfig.Tag, err = api.Parse(originalName, apiConfig.PushAuthentication.ServerAddress)
			if err != nil {
				glog.Errorf("There are some errors in image name, please check the error:\n%v", err)
				return 1
			}
		} else {
			glog.Errorf("not found git")
			return 1
		}
		opts := cmd.CommandOpts{
			Stderr: os.Stderr,
			Stdout: os.Stdout,
		}
		glog.Info(
			"kaniko --host-aliases ", "10.193.28.1:registry.vivo.bj04.xyz",
			"  --dockerfile ", filepath.Join(apiConfig.ContextDir, "Dockerfile"),
			"  --context ", apiConfig.ContextDir,
			"  --skip-tls-verify-registry ",
			apiConfig.PushAuthentication.ServerAddress, "  --destination 	", apiConfig.Tag)
		token := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", apiConfig.PushAuthentication.Username, apiConfig.PushAuthentication.Password)))
		dockerConfig := map[string]interface{}{"auths": map[string]interface{}{
			apiConfig.PushAuthentication.ServerAddress:
			map[string]string{
				"auth": token,
			}}}
		bs, err := json.Marshal(dockerConfig)
		glog.Infof("write docker config [%s] to /kaniko/.docker/", string(bs))
		if err != nil {
			glog.Errorf("marshal docker config err:%v", err)
		}

		if err = os.MkdirAll("/kaniko/.docker/", 0766); err != nil {
			glog.Infof("mkdir dir /kaniko/.docker/ %v", err)
		}
		_file, err := os.Create("/kaniko/.docker/config.json")
		if err != nil {
			glog.Errorf("write docker config failed, %v", err)
		}
		_, err = _file.Write(bs)
		_file.Close()
		if err != nil {
			glog.Errorf("write docker config failed, %v", err)
		}

		glog.Info("docker config path", os.Getenv("DOCKER_CONFIG"))
		err = cmd.NewCommandRunner().RunWithOptions(opts, kanikoPath,
			"--host-aliases", "10.193.28.1:registry.vivo.bj04.xyz",
			"--dockerfile", filepath.Join(apiConfig.ContextDir, "Dockerfile"),
			"--context", apiConfig.ContextDir,
			"--skip-tls-verify-registry", apiConfig.PushAuthentication.ServerAddress,
			"--destination", apiConfig.Tag,
		)
		if err != nil {
			glog.Errorf("Build failed, please check the error:\n%v", err)
			return 1
		}

		ori := strings.Split(originalName, ":")
		imageName := ori[0]
		imageRepoTags := []string{"latest"}
		if len(ori) > 2 {
			imageRepoTags = []string{ori[1]}
		}
		tagInfo := getTagInfo(imageName, imageRepoTags[0], apiConfig.PushAuthentication)
		outputresult.KanikoAddBuildResultToAnnotation(&api.OutputResultInfo{
			ImageName:     imageName,
			ImageCreated:  tagInfo.Created,
			ImageSize:     tagInfo.Size,
			ImageID:       tagInfo.Digest,
			ImageRepoTags: imageRepoTags,
		})
		return 0
	} else {
		glog.Warningf("KanikoEnvVariable is set[%s], but:\n%v", kanikoPath, err)

	}

	apiConfig.Tag, err = api.Parse(apiConfig.Tag, apiConfig.PushAuthentication.ServerAddress)
	if err != nil {
		glog.Errorf("There are some errors in image name, please check the error:\n%v", err)
		return 1
	}
	err = S2I(apiConfig)
	if err != nil {
		glog.Errorf("Build failed, please check the error:\n%v", err)
		return 1
	}
	return 0
}

func getTagInfo(imageName, tag string, entry api.AuthConfig) (tagInfo api.TagInfo) {

	uri := &url.URL{
		Scheme: "https://",
		Host:   entry.ServerAddress,
		Path:   fmt.Sprintf("/api/repositories/%s/tags/%s", imageName, tag),
		User:   url.UserPassword(entry.Username, entry.Password),
	}

	client := http.Client{}
	req, err := http.NewRequest("GET", uri.String(), nil)
	if err != nil {
		glog.Errorf("new req %s failed, %v", uri.String(), err)
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		glog.Errorf(" req %s failed, %v", uri.String(), err)
		return
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&tagInfo)
	if err != nil {
		glog.Errorf("get image failed %+v", err)
	}
	return
}
