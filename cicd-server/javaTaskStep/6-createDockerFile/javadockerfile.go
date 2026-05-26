package createDockerFile

import (
	"cicd-server/config"
	"cicd-server/models"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DockerConfig Docker配置结构
type DockerConfig struct {
	HarborDomain string
	InitImage    string
	JavaVersion  string
	ProjectName  string
	Timestamp    string
}

// ExecuteJavaDockerfile 执行Java项目Dockerfile创建
func ExecuteJavaDockerfile(task *models.Task, projectName, productDir, imageDir, taskLogDir, javaVersion, timestamp string,
	addTaskLog func(*models.Task, string),
	executeCommand func(*models.Task, string) error) error {

	// 检查任务是否被取消
	select {
	case <-task.CancelChan:
		addTaskLog(task, "任务被取消")
		return fmt.Errorf("任务被取消")
	default:
	}

	addTaskLog(task, "开始创建Java项目Dockerfile")

	// 创建Docker配置
	dockerConfig := &DockerConfig{
		HarborDomain: config.GetHarborDomain(),
		JavaVersion:  javaVersion,
		ProjectName:  projectName,
		Timestamp:    timestamp,
	}

	// 获取项目的SkyWalking配置
	enableSkyWalking := false
	if task.ProjectInfo != nil && task.ProjectInfo.EnableSkyWalking {
		enableSkyWalking = true
	}
	addTaskLog(task, fmt.Sprintf("项目 %s SkyWalking配置: %t", projectName, enableSkyWalking))

	// 根据Java版本设置基础镜像
	dockerConfig.InitImage = getInitImage(javaVersion, enableSkyWalking)
	addTaskLog(task, fmt.Sprintf("使用基础镜像: %s", dockerConfig.InitImage))

	// 1. 创建docker-compose.yaml头部
	if err := createComposeHead(task, productDir, imageDir, addTaskLog); err != nil {
		return fmt.Errorf("创建docker-compose头部失败: %v", err)
	}

	// 2. 为每个子项目创建Dockerfile和docker-compose条目
	if err := createJavaDockerfiles(task, dockerConfig, productDir, imageDir, addTaskLog, executeCommand); err != nil {
		return fmt.Errorf("创建Dockerfile失败: %v", err)
	}

	addTaskLog(task, "Java项目Dockerfile创建完成")
	return nil
}

// createComposeHead 创建docker-compose.yaml头部
func createComposeHead(task *models.Task, productDir, imageDir string,
	addTaskLog func(*models.Task, string)) error {

	addTaskLog(task, "开始创建docker-compose头部")

	// 切换到产物目录并获取项目列表
	entries, err := os.ReadDir(productDir)
	if err != nil {
		return fmt.Errorf("读取产物目录失败: %v", err)
	}

	// 过滤出目录并创建项目列表
	var projectList []string
	for _, entry := range entries {
		if entry.IsDir() {
			projectList = append(projectList, entry.Name())
		}
	}

	// 创建projectlist.txt
	projectListFile := filepath.Join(productDir, "projectlist.txt")
	projectListContent := strings.Join(projectList, "\n")
	if err := os.WriteFile(projectListFile, []byte(projectListContent), 0644); err != nil {
		return fmt.Errorf("创建项目列表文件失败: %v", err)
	}

	addTaskLog(task, fmt.Sprintf("本次更新项目有%d个", len(projectList)))

	// 创建docker-compose.yaml
	composeFile := filepath.Join(imageDir, "docker-compose.yaml")
	composeContent := `version: '3'
services:
`
	if err := os.WriteFile(composeFile, []byte(composeContent), 0644); err != nil {
		return fmt.Errorf("创建docker-compose.yaml失败: %v", err)
	}

	addTaskLog(task, "docker-compose头部创建完成")
	return nil
}

// createJavaDockerfiles 为每个Java子项目创建Dockerfile
func createJavaDockerfiles(task *models.Task, config *DockerConfig, productDir, imageDir string,
	addTaskLog func(*models.Task, string),
	executeCommand func(*models.Task, string) error) error {

	// 读取项目列表
	projectListFile := filepath.Join(productDir, "projectlist.txt")
	projectListData, err := os.ReadFile(projectListFile)
	if err != nil {
		return fmt.Errorf("读取项目列表失败: %v", err)
	}

	projectList := strings.Split(strings.TrimSpace(string(projectListData)), "\n")

	// 创建images.txt文件用于记录镜像列表
	imagesFile := filepath.Join(imageDir, "images.txt")
	imagesFileHandle, err := os.Create(imagesFile)
	if err != nil {
		return fmt.Errorf("创建images.txt失败: %v", err)
	}
	defer imagesFileHandle.Close()

	for _, subProject := range projectList {
		if subProject == "" {
			continue
		}

		addTaskLog(task, fmt.Sprintf("开始处理子项目: %s", subProject))

		// 检查任务是否被取消
		select {
		case <-task.CancelChan:
			addTaskLog(task, "任务被取消")
			return fmt.Errorf("任务被取消")
		default:
		}

		// 1. 先更新docker-compose.yaml
		if err := updateDockerCompose(config, subProject, imageDir, imagesFileHandle); err != nil {
			addTaskLog(task, fmt.Sprintf("更新docker-compose失败: %v", err))
			continue
		}

		// 2. 创建子项目镜像目录
		subProjectImageDir := filepath.Join(imageDir, subProject)
		if err := os.MkdirAll(subProjectImageDir, 0755); err != nil {
			addTaskLog(task, fmt.Sprintf("创建子项目镜像目录失败: %v", err))
			continue
		}

		// 3. 移动pkg目录
		pkgSrc := filepath.Join(productDir, subProject, "target", "pkg")
		pkgDst := filepath.Join(subProjectImageDir, "pkg")

		// 如果目标pkg目录已存在，先删除
		if _, err := os.Stat(pkgDst); err == nil {
			if err := os.RemoveAll(pkgDst); err != nil {
				addTaskLog(task, fmt.Sprintf("清理目标pkg目录失败 %s: %v", pkgDst, err))
				continue
			}
		}

		if err := moveDirectory(pkgSrc, pkgDst); err != nil {
			addTaskLog(task, fmt.Sprintf("移动pkg目录失败 %s -> %s: %v", pkgSrc, pkgDst, err))
			continue
		}

		// 4. 查找jar文件
		jarName, err := findJarFile(pkgDst)
		if err != nil {
			addTaskLog(task, fmt.Sprintf("查找jar文件失败: %v", err))
			continue
		}

		// 5. 创建Dockerfile
		if err := createDockerfile(config, subProject, subProjectImageDir, jarName); err != nil {
			addTaskLog(task, fmt.Sprintf("创建Dockerfile失败: %v", err))
			continue
		}

		addTaskLog(task, fmt.Sprintf("子项目 %s 处理完成", subProject))
	}

	return nil
}

// getInitImage 根据Java版本和SkyWalking配置获取基础镜像
func getInitImage(javaVersion string, enableSkyWalking bool) string {
	// 根据Java版本确定镜像版本号
	var imageVersion string
	switch javaVersion {
	case "java8":
		imageVersion = "9.3-openjdk8-zh"
	case "java17":
		imageVersion = "9.3-openjdk17-zh"
	case "java21":
		imageVersion = "9.3-openjdk21-zh"
	default:
		imageVersion = "9.3-openjdk17-zh"
	}

	// 根据项目名称决定是否使用SkyWalking镜像
	switch enableSkyWalking {
	case true:
		return fmt.Sprintf("rocky-skywalking:%s", imageVersion)
	default:
		return fmt.Sprintf("rocky:%s", imageVersion)
	}
}

// createDockerfile 创建单个项目的Dockerfile
func createDockerfile(config *DockerConfig, subProject, subProjectImageDir, jarName string) error {
	dockerfilePath := filepath.Join(subProjectImageDir, "dockerfile")

	// 确定工作目录：对于CRM项目使用projectName，其他项目使用subProject
	workDir := subProject
	if models.IsCrmProject(config.ProjectName) {
		workDir = config.ProjectName
	}

	var dockerfileContent string
	if strings.Contains(config.InitImage, "skywalking") {
		// 使用SkyWalking镜像的Dockerfile
		dockerfileContent = fmt.Sprintf(`FROM %s/library/%s
WORKDIR /data/project/%s
ADD pkg .
ENV JAVA_OPTS="-server -Xms2g -Xmx8g -XX:CompressedClassSpaceSize=2g -XX:MaxMetaspaceSize=2g -XX:+UseG1GC"
ENTRYPOINT exec java -javaagent:/files/skywalking-agent.jar -Dskywalking.collector.backend_service=$SKYWALKING_SERVER -Dskywalking.agent.service_name=$ENVIRONMENT-$SERVICE_NAME -Dskywalking.agent.instance_name=$MY_POD_NAME -jar $JAVA_OPTS %s
`, config.HarborDomain, config.InitImage, workDir, jarName)
	} else {
		// 普通镜像的Dockerfile
		dockerfileContent = fmt.Sprintf(`FROM %s/library/%s
WORKDIR /data/project/%s
ADD pkg .
ENV JAVA_OPTS="-server -Xms2g -Xmx8g -XX:CompressedClassSpaceSize=2g -XX:MaxMetaspaceSize=2g -XX:+UseG1GC"
ENTRYPOINT exec java -jar $JAVA_OPTS %s
`, config.HarborDomain, config.InitImage, workDir, jarName)
	}

	return os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0644)
}

// updateDockerCompose 更新docker-compose.yaml文件和images.txt
func updateDockerCompose(config *DockerConfig, subProject, imageDir string, imagesFileHandle *os.File) error {
	composeFile := filepath.Join(imageDir, "docker-compose.yaml")

	// 构建镜像完整名称：CRM项目使用/项目名/项目名格式，其他项目使用/项目名/子项目名格式
	var imageName string
	if models.IsCrmProject(config.ProjectName) {
		// CRM项目：harbor域名/项目名/项目名:时间戳
		imageName = fmt.Sprintf("%s/%s/%s:%s", config.HarborDomain, config.ProjectName, config.ProjectName, config.Timestamp)
	} else {
		// 其他项目：harbor域名/项目名/子项目名:时间戳
		imageName = fmt.Sprintf("%s/%s/%s:%s", config.HarborDomain, config.ProjectName, subProject, config.Timestamp)
	}

	// 构建docker-compose条目
	composeEntry := fmt.Sprintf(`  %s:
    build:
      context: %s/%s
      dockerfile: dockerfile
    image: %s
`, subProject, imageDir, subProject, imageName)

	// 追加到docker-compose.yaml
	file, err := os.OpenFile(composeFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err = file.WriteString(composeEntry); err != nil {
		return err
	}

	// 写入镜像名称到images.txt
	if _, err = imagesFileHandle.WriteString(imageName + "\n"); err != nil {
		return err
	}

	return nil
}

// 辅助函数
func moveDirectory(src, dst string) error {
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return fmt.Errorf("源目录不存在: %s", src)
	}
	return os.Rename(src, dst)
}

func findJarFile(pkgDir string) (string, error) {
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".jar") && !strings.Contains(entry.Name(), "sources") {
			return entry.Name(), nil
		}
	}

	return "", fmt.Errorf("未找到jar文件")
}
