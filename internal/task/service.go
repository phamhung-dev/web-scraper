package task

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"web-scraper/internal/pkg/models"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
	"github.com/google/uuid"
)

type TaskStorage interface {
	Insert(ctx context.Context, task *models.Task) error
}

type taskService struct {
	taskStorage TaskStorage
}

type taskSummary struct {
	ID  uuid.UUID
	URL string
}

func NewTaskService(taskStorage TaskStorage) *taskService {
	return &taskService{taskStorage: taskStorage}
}

func (t *taskService) ScrapeTasks(ctx context.Context) error {
	opts := []chromedp.ExecAllocatorOption{
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3830.0 Safari/537.36"),
		chromedp.WindowSize(1920, 1080),
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Headless,
		chromedp.DisableGPU,
		chromedp.ProxyServer(os.Getenv("socks_proxy")),
		chromedp.NoSandbox,
		chromedp.Flag("disable-dev-shm-usage", true),
	}

	browserCtx, browserCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer browserCancel()

	ctx1, ctx1Cancel := chromedp.NewContext(browserCtx)
	defer ctx1Cancel()

	webURL := "https://app.any.run/submissions/"

	if err := chromedp.Run(
		ctx1,
		chromedp.Navigate(webURL),
	); err != nil {
		return err
	}

	ctx2, ctx2Cancel := chromedp.NewContext(ctx1)
	defer ctx2Cancel()

	lastPageTasks, err := strconv.Atoi(os.Getenv("LAST_PAGE_TASKS"))
	if err != nil {
		return err
	}

	for i := lastPageTasks; i > 0; i-- {
		if err := chromedp.Run(
			ctx1,
			chromedp.WaitVisible(".history-pagination__input"),
			chromedp.Click(".history-pagination__current-page", chromedp.ByQuery, chromedp.NodeVisible),
		); err != nil {
			return err
		}
		var inputValue string
		for {
			if err := chromedp.Run(
				ctx1,
				chromedp.SendKeys(".history-pagination__input", kb.Backspace, chromedp.ByQuery),
				chromedp.Value("input[class=history-pagination__input]", &inputValue, chromedp.ByQuery),
			); err != nil {
				return err
			}
			if len(inputValue) == 0 {
				break
			}
		}
		inputValue = strconv.Itoa(i)
		for _, value := range inputValue {
			if err := chromedp.Run(
				ctx1,
				chromedp.SendKeys(".history-pagination__input", string(value), chromedp.ByQuery),
			); err != nil {
				return err
			}
		}
		if err := chromedp.Run(
			ctx1,
			chromedp.SendKeys(".history-pagination__input", kb.Enter, chromedp.ByQuery),
			chromedp.Sleep(1*time.Second),
		); err != nil {
			return err
		}

		taskSummaries, err := scrapeTaskSummaries(ctx1, webURL)
		if err != nil {
			return err
		}

		for _, taskSummary := range taskSummaries {
			task, err := scrapeTask(ctx2, taskSummary.ID, taskSummary.URL)
			if err != nil {
				fmt.Printf("scrape: response: %s at %s\n", err, taskSummary.URL)
				continue
			}
			if err := t.taskStorage.Insert(ctx2, task); err != nil {
				fmt.Println(err)
				continue
			}
		}
	}

	return nil
}

func scrapeTaskSummaries(ctx context.Context, webURL string) ([]taskSummary, error) {
	var taskNodes []*cdp.Node

	if err := chromedp.Run(
		ctx,
		chromedp.WaitVisible(".history-table--content"),
		chromedp.Nodes(".history-table--content__row", &taskNodes, chromedp.ByQueryAll, chromedp.AtLeast(0)),
	); err != nil {
		return nil, err
	}

	var (
		href          string
		id            uuid.UUID
		sha256        string
		taskSummaries []taskSummary
	)

	for _, taskNode := range taskNodes {
		if err := chromedp.Run(
			ctx,
			chromedp.AttributeValue("a", "href", &href, nil, chromedp.ByQuery, chromedp.FromNode(taskNode)),
			chromedp.Text("a .history__hash.noselect .hash__item:nth-child(3) .hash__value", &sha256, chromedp.ByQueryAll, chromedp.FromNode(taskNode), chromedp.AtLeast(0)),
		); err != nil {
			continue
		}

		href = strings.TrimSpace(href)
		sha256 = strings.ToLower(strings.TrimSpace(sha256))
		id, _ = uuid.Parse(strings.Split(href, "/")[2])

		taskSummaries = append(
			taskSummaries,
			taskSummary{
				ID:  id,
				URL: fmt.Sprintf("https://any.run/report/%s/%s\n", sha256, id),
			},
		)
	}

	return taskSummaries, nil
}

func scrapeTask(ctx context.Context, taskID uuid.UUID, taskURL string) (*models.Task, error) {
	if err := chromedp.Run(
		ctx,
		chromedp.Navigate(taskURL),
		chromedp.WaitVisible(".App_ctn_yjkFn"),
	); err != nil {
		return nil, err
	}

	task := models.Task{}
	task.ID = taskID
	wg := sync.WaitGroup{}
	wg.Add(10)
	go scrapeGeneral(ctx, &task, &wg)
	go scrapeBehaviorActivities(ctx, &task, &wg)
	go scrapeMalwareConfigurations(ctx, &task, &wg)
	go scrapeStaticInformation(ctx, &task, &wg)
	go scrapeVideoAndScreenshots(ctx, &task, &wg)
	go scrapeProcesses(ctx, &task, &wg)
	go scrapeRegistryActivity(ctx, &task, &wg)
	go scrapeFilesActivity(ctx, &task, &wg)
	go scrapeNetworkActivity(ctx, &task, &wg)
	go scrapeDebugOutputStrings(ctx, &task, &wg)
	wg.Wait()

	return &task, nil
}

func scrapeGeneral(ctx context.Context, task *models.Task, wg *sync.WaitGroup) {
	defer wg.Done()

	var (
		infoNodes                []*cdp.Node
		launchConfigurationNodes []*cdp.Node
	)

	if err := chromedp.Run(
		ctx,
		chromedp.Nodes(".General_generalTable_dLUUW tbody tr", &infoNodes, chromedp.ByQueryAll, chromedp.AtLeast(0)),
		chromedp.Nodes(".General_softBody_E0W8S", &launchConfigurationNodes, chromedp.ByQuery, chromedp.AtLeast(0)),
	); err != nil {
		return
	}

	info := models.Info{}
	scrapeGeneralNameValueText(ctx, &infoNodes, ".General_name_i6WsV", ".General_valueTextSpan_k7WBm", &info)
	scrapeGeneralNameValueTextArray(ctx, &infoNodes, ".General_name_i6WsV", ".TabsViewer_tabs_stA_S.TabsViewer_mainTabs_pQOEz .TabsViewer_tab_Egwa8", &info)
	scrapeGeneralNameValueTextArray(ctx, &infoNodes, ".General_name_i6WsV", ".General_tag_Q_RAi", &info)
	scrapeGeneralNameValueAttributeArray(ctx, &infoNodes, ".General_name_i6WsV", ".General_indicators_SwycP .Indicator_svg_z3nJn", "title", &info)
	task.General.Info = info

	launchConfiguration := models.LaunchConfiguration{}
	scrapeLaunchConfigurationNameValueText(ctx, &launchConfigurationNodes, ".General_keyValueGrid_fCvjh *", &launchConfiguration)
	scrapeLaunchConfigurationNameValueTextArray(ctx, &launchConfigurationNodes, ".General_listTitle_Uwa3t", ".common-module_list_XpRd_ *", &launchConfiguration)
	task.General.LaunchConfiguration = launchConfiguration
}

func scrapeBehaviorActivities(ctx context.Context, task *models.Task, wg *sync.WaitGroup) {
	defer wg.Done()

	var infoNodes []*cdp.Node

	if err := chromedp.Run(
		ctx,
		chromedp.Nodes(".BehaviorActivities_ctn_VZYWX .BehaviorActivities_lists_aDTi7 > li", &infoNodes, chromedp.ByQueryAll, chromedp.AtLeast(0)),
	); err != nil {
		return
	}

	malicious := models.BehaviorActivity{}
	scrapeBehaviorActivityNameValueTextArray(ctx, infoNodes[0], ".BehaviorActivities_listSubTile_JXjra", ".BehaviorActivities_listSubValues_vPfix.common-module_list_XpRd_ > li", &malicious)
	task.BehaviorActivities.Malicious = malicious

	suspicious := models.BehaviorActivity{}
	scrapeBehaviorActivityNameValueTextArray(ctx, infoNodes[1], ".BehaviorActivities_listSubTile_JXjra", ".BehaviorActivities_listSubValues_vPfix.common-module_list_XpRd_ > li", &suspicious)
	task.BehaviorActivities.Suspicious = suspicious

	info := models.BehaviorActivity{}
	scrapeBehaviorActivityNameValueTextArray(ctx, infoNodes[2], ".BehaviorActivities_listSubTile_JXjra", ".BehaviorActivities_listSubValues_vPfix.common-module_list_XpRd_ > li", &info)
	task.BehaviorActivities.Info = info
}

func scrapeMalwareConfigurations(ctx context.Context, task *models.Task, wg *sync.WaitGroup) {
	defer wg.Done()
	malwareConfigurations := models.MalwareConfigurations{}
	scrapeMalwareConfigurationNameValueText(ctx, nil, ".MalwareConfiguration_ctn_g3Obw > *:not(:last-child)", "MalwareConfiguration_familyName_Ldd65", &malwareConfigurations)
	task.MalwareConfigurations = malwareConfigurations
}

func scrapeStaticInformation(ctx context.Context, task *models.Task, wg *sync.WaitGroup) {
	defer wg.Done()

	staticInformation := models.StaticInformation{}
	scrapeStaticInformationNameValueText(ctx, &staticInformation)
	task.StaticInformation = staticInformation
}

func scrapeVideoAndScreenshots(ctx context.Context, task *models.Task, wg *sync.WaitGroup) {
	defer wg.Done()

	videoAndScreenshots := models.VideoAndScreenshots{}
	scrapeVideoAndScreenshotsNameValueTextArray(ctx, &videoAndScreenshots)
	task.VideoAndScreenshots = videoAndScreenshots
}

func scrapeProcesses(ctx context.Context, task *models.Task, wg *sync.WaitGroup) {
	defer wg.Done()

	processes := models.Processes{}
	scrapeProcessNameValueText(ctx, &processes)
	task.Processes = processes
}

func scrapeRegistryActivity(ctx context.Context, task *models.Task, wg *sync.WaitGroup) {
	defer wg.Done()

	registryActivity := models.RegistryActivity{}
	scrapeRegistryActivityNameValueText(ctx, &registryActivity)
	task.RegistryActivity = registryActivity
}

func scrapeFilesActivity(ctx context.Context, task *models.Task, wg *sync.WaitGroup) {
	defer wg.Done()

	filesActivity := models.FilesActivity{}
	scrapeFilesActivityNameValueText(ctx, &filesActivity)
	task.FilesActivity = filesActivity
}

func scrapeNetworkActivity(ctx context.Context, task *models.Task, wg *sync.WaitGroup) {
	defer wg.Done()

	networkActivity := models.NetworkActivity{}
	scrapeNetworkActivityNameValueText(ctx, &networkActivity)
	task.NetworkActivity = networkActivity
}

func scrapeDebugOutputStrings(ctx context.Context, task *models.Task, wg *sync.WaitGroup) {
	defer wg.Done()

	debugOutputStrings := models.DebugOutputStrings{}
	scrapeDebugOutputStringsNameValueText(ctx, &debugOutputStrings)
	task.DebugOutputStrings = debugOutputStrings
}

func scrapeGeneralNameValueText(ctx context.Context, infoNodes *[]*cdp.Node, nameQuery string, valueTextQuery string, info *models.Info) {
	var (
		tempInfoNodes []*cdp.Node
		name, value   string
	)

	for _, infoNode := range *infoNodes {
		if err := chromedp.Run(
			ctx,
			chromedp.Text(valueTextQuery, &value, chromedp.ByQuery, chromedp.FromNode(infoNode), chromedp.AtLeast(0)),
		); err != nil {
			tempInfoNodes = append(tempInfoNodes, infoNode)
			continue
		}
		if err := chromedp.Run(
			ctx,
			chromedp.Text(nameQuery, &name, chromedp.ByQuery, chromedp.FromNode(infoNode), chromedp.AtLeast(0)),
		); err != nil {
			continue
		}

		name = toSnakeCase(name)
		(*info)[name] = value
	}

	*infoNodes = tempInfoNodes
}

func scrapeGeneralNameValueTextArray(ctx context.Context, infoNodes *[]*cdp.Node, nameQuery string, valueTextQuery string, info *models.Info) {
	var (
		tempInfoNodes  []*cdp.Node
		valueTextNodes []*cdp.Node
		name           string
		value          []string
	)

	for _, infoNode := range *infoNodes {
		if err := chromedp.Run(
			ctx,
			chromedp.Nodes(valueTextQuery, &valueTextNodes, chromedp.ByQueryAll, chromedp.FromNode(infoNode), chromedp.AtLeast(0)),
		); err != nil || len(valueTextNodes) < 1 {
			tempInfoNodes = append(tempInfoNodes, infoNode)
			continue
		}

		for _, valueTextNode := range valueTextNodes {
			value = append(value, valueTextNode.Children[0].NodeValue)
		}

		if err := chromedp.Run(
			ctx,
			chromedp.Text(nameQuery, &name, chromedp.ByQuery, chromedp.FromNode(infoNode), chromedp.AtLeast(0)),
		); err != nil {
			continue
		}

		name = toSnakeCase(name)
		(*info)[name] = value
	}

	*infoNodes = tempInfoNodes
}

func scrapeGeneralNameValueAttributeArray(ctx context.Context, infoNodes *[]*cdp.Node, nameQuery string, valueAttributeQuery string, attributeName string, info *models.Info) {
	var (
		tempInfoNodes       []*cdp.Node
		valueAttributeNodes []*cdp.Node
		name                string
		value               []string
	)

	for _, infoNode := range *infoNodes {
		if err := chromedp.Run(
			ctx,
			chromedp.Nodes(valueAttributeQuery, &valueAttributeNodes, chromedp.ByQueryAll, chromedp.FromNode(infoNode), chromedp.AtLeast(0)),
		); err != nil || len(valueAttributeNodes) < 1 {
			tempInfoNodes = append(tempInfoNodes, infoNode)
			continue
		}

		for _, valueAttributeNode := range valueAttributeNodes {
			value = append(value, valueAttributeNode.AttributeValue(attributeName))
		}

		if err := chromedp.Run(
			ctx,
			chromedp.Text(nameQuery, &name, chromedp.ByQuery, chromedp.FromNode(infoNode), chromedp.AtLeast(0)),
		); err != nil {
			continue
		}

		name = toSnakeCase(name)
		(*info)[name] = value
	}

	*infoNodes = tempInfoNodes
}

func scrapeLaunchConfigurationNameValueText(ctx context.Context, launchConfigurationNodes *[]*cdp.Node, valueTextQuery string, launchConfiguration *models.LaunchConfiguration) {
	if len(*launchConfigurationNodes) != 1 {
		return
	}

	var (
		launchConfigurationInfoNodes []*cdp.Node
		name, value                  string
	)

	if err := chromedp.Run(
		ctx,
		chromedp.Nodes(valueTextQuery, &launchConfigurationInfoNodes, chromedp.ByQueryAll, chromedp.FromNode((*launchConfigurationNodes)[0]), chromedp.AtLeast(0)),
	); err != nil {
		return
	}

	launchConfigurationInfoNodesLength := len(launchConfigurationInfoNodes)
	for i := 0; i < launchConfigurationInfoNodesLength; i += 2 {
		name = toSnakeCase(launchConfigurationInfoNodes[i].Children[0].NodeValue)
		value = launchConfigurationInfoNodes[i+1].Children[0].NodeValue

		if name != "" && value != "" {
			(*launchConfiguration)[name] = value
		}
	}
}

func scrapeLaunchConfigurationNameValueTextArray(ctx context.Context, launchConfigurationNodes *[]*cdp.Node, nameQuery string, valueTextQuery string, launchConfiguration *models.LaunchConfiguration) {
	if len(*launchConfigurationNodes) != 1 {
		return
	}

	var (
		launchConfigurationInfoNodes []*cdp.Node
		tempInfoNodes                []*cdp.Node
		name                         string
		value                        []string
	)

	if err := chromedp.Run(
		ctx,
		chromedp.Nodes(".General_lists_sVr1a div", &launchConfigurationInfoNodes, chromedp.ByQueryAll, chromedp.FromNode((*launchConfigurationNodes)[0]), chromedp.AtLeast(0)),
	); err != nil {
		return
	}

	for _, launchConfigurationInfoNode := range launchConfigurationInfoNodes {
		value = []string{}

		if err := chromedp.Run(
			ctx,
			chromedp.TextContent(nameQuery, &name, chromedp.ByQuery, chromedp.FromNode(launchConfigurationInfoNode), chromedp.AtLeast(0)),
			chromedp.Nodes(valueTextQuery, &tempInfoNodes, chromedp.ByQueryAll, chromedp.FromNode(launchConfigurationInfoNode), chromedp.AtLeast(0)),
		); err != nil {
			continue
		}

		for _, tempInfoNode := range tempInfoNodes {
			value = append(value, tempInfoNode.Children[0].NodeValue)
		}

		if len(value) > 0 {
			name = toSnakeCase(name)
			(*launchConfiguration)[name] = value
		}
	}
}

func scrapeBehaviorActivityNameValueTextArray(ctx context.Context, currNode *cdp.Node, nameQuery string, valueTextQuery string, behaviorActivity *models.BehaviorActivity) {
	var (
		title          string
		activities     []string
		infoNodes      []*cdp.Node
		childInfoNodes []*cdp.Node
		value          []map[string]interface{}
	)

	if err := chromedp.Run(
		ctx,
		chromedp.Nodes(".BehaviorActivities_listValues_xdw55 > li", &infoNodes, chromedp.ByQueryAll, chromedp.FromNode(currNode), chromedp.AtLeast(0)),
	); err != nil {
		return
	}

	for _, infoNode := range infoNodes {
		activities = []string{}
		if err := chromedp.Run(
			ctx,
			chromedp.Text(nameQuery, &title, chromedp.ByQuery, chromedp.FromNode(infoNode), chromedp.AtLeast(0)),
			chromedp.Nodes(valueTextQuery, &childInfoNodes, chromedp.ByQueryAll, chromedp.FromNode(infoNode), chromedp.AtLeast(0)),
		); err != nil {
			continue
		}

		for _, childInfoNode := range childInfoNodes {
			activities = append(activities, childInfoNode.Children[0].NodeValue)
		}

		activity := map[string]interface{}{
			"title":      title,
			"activities": activities,
		}

		value = append(value, activity)
	}

	*behaviorActivity = value
}

func scrapeMalwareConfigurationNameValueText(ctx context.Context, currNode *cdp.Node, configNodesQuery string, familyNameAttributeClass string, malwareConfigurations *models.MalwareConfigurations) {
	var (
		malwareConfigurationsNodes []*cdp.Node
		infoNodes                  []*cdp.Node
		malwareConfiguration       models.MalwareConfiguration
		key, value, prevKey        string
		values                     []string
		numberOfJumps              = 1
		count                      = 0
		info                       = make(map[string]interface{})
	)

	if err := chromedp.Run(
		ctx,
		chromedp.Nodes(configNodesQuery, &malwareConfigurationsNodes, chromedp.ByQueryAll, chromedp.FromNode(currNode), chromedp.AtLeast(0)),
	); err != nil || len(malwareConfigurationsNodes) < 2 {
		return
	}

	malwareConfigurationsNodesLength := len(malwareConfigurationsNodes)
	for i := 0; i < malwareConfigurationsNodesLength; i++ {
		if strings.Contains(malwareConfigurationsNodes[i].AttributeValue("class"), familyNameAttributeClass) {
			if len(malwareConfiguration.MalwareConfigurationInfo) > 0 {
				*malwareConfigurations = append(*malwareConfigurations, malwareConfiguration)
			}
			malwareConfiguration = models.MalwareConfiguration{}
			malwareConfiguration.FamilyName = malwareConfigurationsNodes[i].Children[0].NodeValue
			continue
		}

		if err := chromedp.Run(
			ctx,
			chromedp.Nodes(".TableViewerLine_line_a0PEx", &infoNodes, chromedp.ByQueryAll, chromedp.FromNode(malwareConfigurationsNodes[i]), chromedp.AtLeast(0)),
		); err != nil {
			continue
		}

		for _, infoNode := range infoNodes {
			if err := chromedp.Run(
				ctx,
				chromedp.Text(".TableViewerLine_key__FuBF", &key, chromedp.ByQuery, chromedp.FromNode(infoNode), chromedp.AtLeast(0)),
				chromedp.Text(".TableViewerLine_value_OhSTs > span", &value, chromedp.ByQuery, chromedp.FromNode(infoNode), chromedp.AtLeast(0)),
			); err != nil {
				continue
			}
			tempNumberOfJumps := getNumOfJumpsFromKey(key)
			firstIndexRoundBracket := strings.Index(key, "(")
			if firstIndexRoundBracket > 0 {
				key = key[:firstIndexRoundBracket-1]
			}
			if tempNumberOfJumps > 1 {
				numberOfJumps = tempNumberOfJumps
				prevKey = toSnakeCase(key)
			}
			if numberOfJumps > 1 {
				values = append(values, value)
				count++
			} else {
				key = toSnakeCase(key)
				info[key] = value
			}

			if count == numberOfJumps {
				info[prevKey] = values
				values = []string{}
				numberOfJumps = 1
				count = 0
			}
		}
		malwareConfiguration.MalwareConfigurationInfo = append(malwareConfiguration.MalwareConfigurationInfo, info)
		info = make(map[string]interface{})
	}
	if len(malwareConfiguration.MalwareConfigurationInfo) > 0 {
		*malwareConfigurations = append(*malwareConfigurations, malwareConfiguration)
	}
}

func scrapeStaticInformationNameValueText(ctx context.Context, staticInformation *models.StaticInformation) {
	var (
		staticInformationNodes []*cdp.Node
		infoNodes              []*cdp.Node
		name, value            string
		trid                   models.TRiD
		exif                   models.EXIF
	)

	if err := chromedp.Run(
		ctx,
		chromedp.Nodes(".StaticInformation_content_d5cVu.StaticInformation_contentSimple_brbit .StaticInformation_simpleTable_Dtqrc tbody", &staticInformationNodes, chromedp.ByQueryAll, chromedp.AtLeast(0)),
	); err != nil || len(staticInformationNodes) == 0 {
		return
	}

	if err := chromedp.Run(
		ctx,
		chromedp.Nodes("*", &infoNodes, chromedp.ByQueryAll, chromedp.FromNode(staticInformationNodes[0]), chromedp.AtLeast(0)),
	); err != nil {
		return
	}

	for _, infoNode := range infoNodes {
		if err := chromedp.Run(
			ctx,
			chromedp.Text(".StaticInformation_tridExt_yOLSW", &name, chromedp.ByQuery, chromedp.FromNode(infoNode), chromedp.AtLeast(0)),
			chromedp.Text(".StaticInformation_tridValue_ao7XH", &value, chromedp.ByQuery, chromedp.FromNode(infoNode), chromedp.AtLeast(0)),
		); err != nil {
			continue
		}

		tridInfo := models.TRiDInfo{
			Ext:   name,
			Value: value,
		}

		trid = append(trid, tridInfo)
	}

	(*staticInformation).TRiD = trid

	if len(staticInformationNodes) == 2 {
		if err := chromedp.Run(
			ctx,
			chromedp.Nodes("*", &infoNodes, chromedp.ByQueryAll, chromedp.FromNode(staticInformationNodes[1]), chromedp.AtLeast(0)),
		); err != nil {
			return
		}

		for _, infoNode := range infoNodes {
			if err := chromedp.Run(
				ctx,
				chromedp.Text(".common-module_keyValueName_toYfX.StaticInformation_keyValueName__doOb", &name, chromedp.ByQuery, chromedp.FromNode(infoNode), chromedp.AtLeast(0)),
				chromedp.Text(".common-module_keyValueValue_rvzCr.StaticInformation_mayBeList_QOXNG", &value, chromedp.ByQuery, chromedp.FromNode(infoNode), chromedp.AtLeast(0)),
			); err != nil {
				continue
			}

			exifInfo := models.EXIFInfo{
				Name:  name,
				Value: value,
			}

			exif = append(exif, exifInfo)
		}

		(*staticInformation).EXIF = exif
	}
}

func scrapeVideoAndScreenshotsNameValueTextArray(ctx context.Context, videoAndScreenshots *models.VideoAndScreenshots) {
	var (
		videoAndScreenshotsNodes []*cdp.Node
		urls                     models.URLs
	)

	if err := chromedp.Run(
		ctx,
		chromedp.Nodes(".ScreenshotsSwiper_contentCtn_oYxP1 > .ScreenshotsSwiper_content_uKpuG > img", &videoAndScreenshotsNodes, chromedp.ByQueryAll, chromedp.AtLeast(0)),
	); err != nil || len(videoAndScreenshotsNodes) == 0 {
		return
	}

	for _, videoAndScreenshotsNode := range videoAndScreenshotsNodes {
		urls = append(urls, videoAndScreenshotsNode.AttributeValue("src"))
	}

	(*videoAndScreenshots).URLs = urls
}

func scrapeProcessNameValueText(ctx context.Context, processes *models.Processes) {
	var processesNodes []*cdp.Node
	if err := chromedp.Run(
		ctx,
		chromedp.Nodes(".Processes_ctn_Pu12n .Stats_stats_PLanP.Processes_stats_P5pOl .Stats_stat_KvyBP .Stats_statValue_btM0S", &processesNodes, chromedp.ByQueryAll, chromedp.AtLeast(0)),
	); err != nil || len(processesNodes) != 4 {
		(*processes).Total = processesNodes[0].Children[0].NodeValue
		(*processes).Monitored = processesNodes[1].Children[0].NodeValue
		(*processes).Malicious = processesNodes[2].Children[0].NodeValue
		(*processes).Suspicious = processesNodes[3].Children[0].NodeValue
	}

	processInformations := models.ProcessInformations{}
	pageCount := getPageCountTable(ctx, nil, ".Processes_ctn_Pu12n > .Processes_table_LeKlM > .PaginationTable_controls_bHSjz > div > .Pagination_containerClass_vLdBm > li:nth-last-child(3) > button")
	scrapeProcessInformationsPage(ctx, &processInformations)
	for i := 1; i < pageCount; i++ {
		if err := chromedp.Run(
			ctx,
			chromedp.Click(".Processes_ctn_Pu12n > .Processes_table_LeKlM > .PaginationTable_controls_bHSjz > div > .Pagination_containerClass_vLdBm > li:nth-last-child(2) > button", chromedp.ByQuery, chromedp.NodeVisible),
		); err != nil {
			break
		}
		scrapeProcessInformationsPage(ctx, &processInformations)
	}
	(*processes).ProcessInformations = processInformations
}

func scrapeProcessInformationsPage(ctx context.Context, processInformations *models.ProcessInformations) {
	var (
		rowNodes                                             []*cdp.Node
		tempInfoNodes                                        []*cdp.Node
		infoNodes                                            []*cdp.Node
		pid, cmd, path, indicator, parentProcess, key, value string
		indicators, modules                                  []string
		information                                          map[string]interface{}
		pageCount                                            int
	)

	if err := chromedp.Run(
		ctx,
		chromedp.Nodes(".Processes_ctn_Pu12n > .Processes_table_LeKlM > .media-module_processTableWrapper_C4FAp > .PaginationTable_table_z0qIF > tbody > tr", &rowNodes, chromedp.ByQueryAll, chromedp.AtLeast(0)),
	); err != nil {
		return
	}

	for i := 0; i < len(rowNodes); i += 2 {
		processInformation := models.ProcessInformation{}
		if err := chromedp.Run(
			ctx,
			chromedp.Text("td:nth-child(1) span b", &pid, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(2) span", &cmd, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(3) span", &path, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(5) span", &parentProcess, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
		); err != nil {
			continue
		}
		processInformation["pid"] = pid
		processInformation["cmd"] = cmd
		processInformation["path"] = path
		processInformation["parent_process"] = parentProcess

		if err := chromedp.Run(
			ctx,
			chromedp.Nodes("td:nth-child(4) span span .Indicator_svg_z3nJn", &infoNodes, chromedp.ByQueryAll, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
		); err != nil || len(infoNodes) == 0 {
			if err := chromedp.Run(
				ctx,
				chromedp.Text("td:nth-child(4) span span", &indicator, chromedp.ByQueryAll, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			); err != nil {
				continue
			}
			processInformation["indicators"] = indicator
		} else {
			indicators = []string{}
			for _, infoNode := range infoNodes {
				indicators = append(indicators, infoNode.AttributeValue("title"))
			}
			processInformation["indicators"] = indicators
		}

		if err := chromedp.Run(
			ctx,
			chromedp.Nodes(
				"td:nth-child(1) .Processes_cell_rXIq2 .AccordeonPanel_compact_b6f2V.Processes_accordeon_fXnqE",
				&tempInfoNodes,
				chromedp.ByQueryAll,
				chromedp.FromNode(rowNodes[i+1]),
				chromedp.AtLeast(0),
			),
		); err != nil || len(tempInfoNodes) == 0 {
			continue
		}

		if err := chromedp.Run(
			ctx,
			chromedp.Nodes(
				".AccordeonPanel_body_KnFFB .Processes_information_Xouo6 .Processes_keyValueGrid_AuHGp *",
				&infoNodes,
				chromedp.ByQueryAll,
				chromedp.FromNode(tempInfoNodes[0]),
				chromedp.AtLeast(0),
			),
		); err != nil {
			continue
		}

		information = make(map[string]interface{})
		for j := 0; j < len(infoNodes); j += 2 {
			key = infoNodes[j].Children[0].NodeValue
			key = toSnakeCase(key)
			value = infoNodes[j+1].Children[0].NodeValue
			information[key] = value
		}
		processInformation["information"] = information

		pageCount = getPageCountTable(
			ctx,
			tempInfoNodes[0],
			".AccordeonPanel_body_KnFFB .Processes_information_Xouo6 .Processes_modulesTable_YwRnm .PaginationTable_controls_bHSjz div .Pagination_containerClass_vLdBm li:nth-last-child(3) button",
		)
		modules = []string{}
		scrapeProcessInformationModulesPage(ctx, tempInfoNodes[0], &modules)

		for j := 1; j < pageCount; j++ {
			if err := chromedp.Run(
				ctx,
				chromedp.Click(
					".AccordeonPanel_body_KnFFB .Processes_information_Xouo6 .Processes_modulesTable_YwRnm .PaginationTable_controls_bHSjz div .Pagination_containerClass_vLdBm li:nth-last-child(2) button",
					chromedp.ByQuery,
					chromedp.FromNode(tempInfoNodes[0]),
					chromedp.NodeVisible,
				),
			); err != nil {
				break
			}
			scrapeProcessInformationModulesPage(ctx, tempInfoNodes[0], &modules)
		}
		processInformation["modules"] = modules

		malwareConfigurations := models.MalwareConfigurations{}
		if len(tempInfoNodes) == 2 {
			scrapeMalwareConfigurationNameValueText(ctx, tempInfoNodes[1], ".AccordeonPanel_body_KnFFB .Processes_information_Xouo6 > *", "Processes_familyName_XEkR3", &malwareConfigurations)
			if len(malwareConfigurations) > 0 {
				processInformation["malware_configurations"] = malwareConfigurations
			}
		}

		*processInformations = append(*processInformations, processInformation)
	}
}

func scrapeProcessInformationModulesPage(ctx context.Context, currNode *cdp.Node, modules *[]string) {
	var (
		rowNodes []*cdp.Node
		value    string
	)

	if err := chromedp.Run(
		ctx,
		chromedp.Nodes(
			".AccordeonPanel_body_KnFFB .Processes_information_Xouo6 .Processes_modulesTable_YwRnm div .PaginationTable_table_z0qIF tbody > tr",
			&rowNodes,
			chromedp.ByQueryAll,
			chromedp.FromNode(currNode),
			chromedp.AtLeast(0),
		),
	); err != nil {
		return
	}

	for _, rowNode := range rowNodes {
		if err := chromedp.Run(
			ctx,
			chromedp.Text("td:nth-child(1) .PaginationTable_cell_AhJWd", &value, chromedp.ByQuery, chromedp.FromNode(rowNode), chromedp.AtLeast(0)),
		); err != nil {
			continue
		}
		*modules = append(*modules, value)
	}
}

func scrapeRegistryActivityNameValueText(ctx context.Context, registryActivity *models.RegistryActivity) {
	var (
		registryActivityNodes []*cdp.Node
		pageCount             int
	)

	if err := chromedp.Run(
		ctx,
		chromedp.Nodes(".RegistryActivity_ctn_iaWiK .Stats_stats_PLanP.RegistryActivity_stats_lJymj .Stats_stat_KvyBP .Stats_statValue_btM0S", &registryActivityNodes, chromedp.ByQueryAll, chromedp.AtLeast(0)),
	); err == nil && len(registryActivityNodes) == 4 {
		(*registryActivity).Total = remove0xA0FromString(registryActivityNodes[0].Children[0].NodeValue)
		(*registryActivity).Read = remove0xA0FromString(registryActivityNodes[1].Children[0].NodeValue)
		(*registryActivity).Write = remove0xA0FromString(registryActivityNodes[2].Children[0].NodeValue)
		(*registryActivity).Delete = remove0xA0FromString(registryActivityNodes[3].Children[0].NodeValue)
	}

	modificationEvents := models.ModificationEvents{}
	pageCount = getPageCountTable(ctx, nil, ".RegistryActivity_ctn_iaWiK .RegistryActivity_table_oD5Fv .PaginationTable_controls_bHSjz div .Pagination_containerClass_vLdBm li:nth-last-child(3) button")
	scrapeRegistryActivityModificationEventsPage(ctx, &modificationEvents)
	for i := 1; i < pageCount; i++ {
		if err := chromedp.Run(
			ctx,
			chromedp.Click(".RegistryActivity_ctn_iaWiK .RegistryActivity_table_oD5Fv .PaginationTable_controls_bHSjz div .Pagination_containerClass_vLdBm li:nth-last-child(2) button", chromedp.ByQuery, chromedp.NodeVisible),
		); err != nil {
			break
		}
		scrapeRegistryActivityModificationEventsPage(ctx, &modificationEvents)
	}
	(*registryActivity).ModificationEvents = modificationEvents
}

func scrapeRegistryActivityModificationEventsPage(ctx context.Context, modificationEvents *models.ModificationEvents) {
	var (
		rowNodes                                []*cdp.Node
		pidProcess, operation, value, key, name string
	)

	if err := chromedp.Run(
		ctx,
		chromedp.Nodes(".RegistryActivity_ctn_iaWiK .RegistryActivity_table_oD5Fv .media-module_eventsTableWrapper_OiE11 .PaginationTable_table_z0qIF tbody > tr", &rowNodes, chromedp.ByQueryAll, chromedp.AtLeast(0)),
	); err != nil {
		return
	}

	for i := 0; i < len(rowNodes); i += 3 {
		if err := chromedp.Run(
			ctx,
			chromedp.Text("td:nth-child(2) span", &pidProcess, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(4) span", &key, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(2) span", &operation, chromedp.ByQuery, chromedp.FromNode(rowNodes[i+1]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(4) span", &name, chromedp.ByQuery, chromedp.FromNode(rowNodes[i+1]), chromedp.AtLeast(0)),
			chromedp.Text("td div .RegistryActivity_spanValue_Y1Iu9 div:nth-child(2)", &value, chromedp.ByQuery, chromedp.FromNode(rowNodes[i+2]), chromedp.AtLeast(0)),
		); err != nil {
			continue
		}
		pidProcess = remove0xA0FromString(pidProcess)
		modificationEvent := models.ModificationEvent{
			PIDProcess: pidProcess,
			Operation:  operation,
			Value:      value,
			Key:        key,
			Name:       name,
		}

		(*modificationEvents) = append((*modificationEvents), modificationEvent)
	}
}

func scrapeFilesActivityNameValueText(ctx context.Context, filesActivity *models.FilesActivity) {
	var (
		filesActivityNodes []*cdp.Node
		pageCount          int
	)

	if err := chromedp.Run(
		ctx,
		chromedp.Nodes(".FilesActivity_ctn_u4OlD .Stats_stats_PLanP.FilesActivity_stats_kju_H .Stats_stat_KvyBP .Stats_statValue_btM0S", &filesActivityNodes, chromedp.ByQueryAll, chromedp.AtLeast(0)),
	); err == nil && len(filesActivityNodes) == 4 {
		(*filesActivity).Executable = filesActivityNodes[0].Children[0].NodeValue
		(*filesActivity).Suspicious = filesActivityNodes[1].Children[0].NodeValue
		(*filesActivity).Text = filesActivityNodes[2].Children[0].NodeValue
		(*filesActivity).Unknown = filesActivityNodes[3].Children[0].NodeValue
	}

	droppedFiles := models.DroppedFiles{}
	pageCount = getPageCountTable(ctx, nil, ".FilesActivity_ctn_u4OlD .FilesActivity_table_RbF_4 .PaginationTable_controls_bHSjz div .Pagination_containerClass_vLdBm li:nth-last-child(3) button")
	scrapeFilesActivityDroppedFilesPage(ctx, &droppedFiles)
	for i := 1; i < pageCount; i++ {
		if err := chromedp.Run(
			ctx,
			chromedp.Click(".FilesActivity_ctn_u4OlD .FilesActivity_table_RbF_4 .PaginationTable_controls_bHSjz div .Pagination_containerClass_vLdBm li:nth-last-child(2) button", chromedp.ByQuery, chromedp.NodeVisible),
		); err != nil {
			break
		}
		scrapeFilesActivityDroppedFilesPage(ctx, &droppedFiles)
	}
	(*filesActivity).DroppedFiles = droppedFiles
}

func scrapeFilesActivityDroppedFilesPage(ctx context.Context, droppedFiles *models.DroppedFiles) {
	var (
		rowNodes                                      []*cdp.Node
		pid, process, filename, filetype, md5, sha256 string
	)

	if err := chromedp.Run(
		ctx,
		chromedp.Nodes(".FilesActivity_ctn_u4OlD .FilesActivity_table_RbF_4 .media-module_filesTableWrapper_D0pyx .PaginationTable_table_z0qIF tbody > tr", &rowNodes, chromedp.ByQueryAll, chromedp.AtLeast(0)),
	); err != nil {
		return
	}

	for i := 0; i < len(rowNodes); i += 2 {
		if err := chromedp.Run(
			ctx,
			chromedp.Text("td:nth-child(1) span b", &pid, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(2) span", &process, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(3) span", &filename, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(4) span span", &filetype, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(2) div .FilesActivity_spanValue_Y7ELd span", &md5, chromedp.ByQuery, chromedp.FromNode(rowNodes[i+1]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(3) div .FilesActivity_spanValue_Y7ELd span", &sha256, chromedp.ByQuery, chromedp.FromNode(rowNodes[i+1]), chromedp.AtLeast(0)),
		); err != nil {
			continue
		}

		droppedFile := models.DroppedFile{
			PID:      pid,
			Process:  process,
			Filename: filename,
			Type:     filetype,
			MD5:      md5,
			SHA256:   sha256,
		}

		(*droppedFiles) = append((*droppedFiles), droppedFile)
	}
}

func scrapeNetworkActivityNameValueText(ctx context.Context, networkActivity *models.NetworkActivity) {
	var (
		infoNodes     []*cdp.Node
		pageCount     int
		lastPageQuery = ".PaginationTable_controls_bHSjz div .Pagination_containerClass_vLdBm li:nth-last-child(3) button"
	)

	if err := chromedp.Run(
		ctx,
		chromedp.Nodes(".NetworkActivity_ctn_XB7Dg .Stats_stats_PLanP.NetworkActivity_stats_nxRan .Stats_stat_KvyBP .Stats_statValue_btM0S", &infoNodes, chromedp.ByQueryAll, chromedp.AtLeast(0)),
	); err == nil && len(infoNodes) == 4 {
		(*networkActivity).HTTPOrHTTPS = infoNodes[0].Children[0].NodeValue
		(*networkActivity).TCPOrUDP = infoNodes[1].Children[0].NodeValue
		(*networkActivity).DNS = infoNodes[2].Children[0].NodeValue
		(*networkActivity).Threats = infoNodes[3].Children[0].NodeValue
	}

	if err := chromedp.Run(
		ctx,
		chromedp.Nodes(".NetworkActivity_ctn_XB7Dg .NetworkActivity_table_bjNrP", &infoNodes, chromedp.ByQueryAll, chromedp.AtLeast(0)),
	); err != nil {
		return
	}

	httpOrhttpsRequests := models.HTTPOrHTTPSRequests{}
	pageCount = getPageCountTable(ctx, infoNodes[0], lastPageQuery)
	scrapeNetworkActivityHTTPOrHTTPSRequestsPage(ctx, infoNodes[0], &httpOrhttpsRequests)
	for i := 1; i < pageCount; i++ {
		if err := chromedp.Run(
			ctx,
			chromedp.Click(".PaginationTable_controls_bHSjz div .Pagination_containerClass_vLdBm li:nth-last-child(2) button", chromedp.ByQuery, chromedp.FromNode(infoNodes[0]), chromedp.NodeVisible),
		); err != nil {
			break
		}
		scrapeNetworkActivityHTTPOrHTTPSRequestsPage(ctx, infoNodes[0], &httpOrhttpsRequests)
	}
	(*networkActivity).HTTPOrHTTPSRequests = httpOrhttpsRequests

	tcpOrudpConnections := models.TCPOrUDPConnections{}
	pageCount = getPageCountTable(ctx, infoNodes[1], lastPageQuery)
	scrapeNetworkActivityTCPOrUDPConnectionsPage(ctx, infoNodes[1], &tcpOrudpConnections)
	for i := 1; i < pageCount; i++ {
		if err := chromedp.Run(
			ctx,
			chromedp.Click(".PaginationTable_controls_bHSjz div .Pagination_containerClass_vLdBm li:nth-last-child(2) button", chromedp.ByQuery, chromedp.FromNode(infoNodes[1]), chromedp.NodeVisible),
		); err != nil {
			break
		}
		scrapeNetworkActivityTCPOrUDPConnectionsPage(ctx, infoNodes[1], &tcpOrudpConnections)
	}
	(*networkActivity).TCPOrUDPConnections = tcpOrudpConnections

	dnsRequests := models.DNSRequests{}
	pageCount = getPageCountTable(ctx, infoNodes[2], lastPageQuery)
	scrapeNetworkActivityDNSRequestsPage(ctx, infoNodes[2], &dnsRequests)
	for i := 1; i < pageCount; i++ {
		if err := chromedp.Run(
			ctx,
			chromedp.Click(".PaginationTable_controls_bHSjz div .Pagination_containerClass_vLdBm li:nth-last-child(2) button", chromedp.ByQuery, chromedp.FromNode(infoNodes[2]), chromedp.NodeVisible),
		); err != nil {
			break
		}
		scrapeNetworkActivityDNSRequestsPage(ctx, infoNodes[2], &dnsRequests)
	}
	(*networkActivity).DNSRequests = dnsRequests

	threatConnections := models.ThreatConnections{}
	pageCount = getPageCountTable(ctx, infoNodes[3], lastPageQuery)
	scrapeNetworkThreatConnectionsPage(ctx, infoNodes[3], &threatConnections)
	for i := 1; i < pageCount; i++ {
		if err := chromedp.Run(
			ctx,
			chromedp.Click(".PaginationTable_controls_bHSjz div .Pagination_containerClass_vLdBm li:nth-last-child(2) button", chromedp.ByQuery, chromedp.FromNode(infoNodes[3]), chromedp.NodeVisible),
		); err != nil {
			break
		}
		scrapeNetworkThreatConnectionsPage(ctx, infoNodes[3], &threatConnections)
	}
	(*networkActivity).ThreatConnections = threatConnections
}

func scrapeNetworkActivityHTTPOrHTTPSRequestsPage(ctx context.Context, currNode *cdp.Node, httpOrhttpsRequests *models.HTTPOrHTTPSRequests) {
	var (
		rowNodes                                                                       []*cdp.Node
		pid, process, method, httpCode, ip, url, cn, httpRequestType, size, reputation string
	)

	if err := chromedp.Run(
		ctx,
		chromedp.Nodes(".media-module_networkTableWrapper_HlF9E .PaginationTable_table_z0qIF tbody > tr", &rowNodes, chromedp.ByQueryAll, chromedp.FromNode(currNode), chromedp.AtLeast(0)),
	); err != nil {
		return
	}

	for i := 0; i < len(rowNodes); i++ {
		if err := chromedp.Run(
			ctx,
			chromedp.Text("td:nth-child(1) .PaginationTable_cell_AhJWd b", &pid, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(2) .PaginationTable_cell_AhJWd", &process, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(3) .PaginationTable_cell_AhJWd", &method, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(4) .PaginationTable_cell_AhJWd", &httpCode, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(5) .PaginationTable_cell_AhJWd", &ip, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(6) .PaginationTable_cell_AhJWd", &url, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(7) .PaginationTable_cell_AhJWd", &cn, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(8) .PaginationTable_cell_AhJWd span span", &httpRequestType, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(9) .PaginationTable_cell_AhJWd", &size, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(10) .PaginationTable_cell_AhJWd span span", &reputation, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
		); err != nil {
			continue
		}

		httpOrhttpsRequest := models.HTTPOrHTTPSRequest{
			PID:        pid,
			Process:    process,
			Method:     method,
			HTTPCode:   httpCode,
			IP:         ip,
			URL:        url,
			CN:         cn,
			Type:       httpRequestType,
			Size:       size,
			Reputation: reputation,
		}

		(*httpOrhttpsRequests) = append((*httpOrhttpsRequests), httpOrhttpsRequest)
	}
}

func scrapeNetworkActivityTCPOrUDPConnectionsPage(ctx context.Context, currNode *cdp.Node, tcpOrudpConnections *models.TCPOrUDPConnections) {
	var (
		rowNodes                                      []*cdp.Node
		pid, process, ip, domain, asn, cn, reputation string
	)

	if err := chromedp.Run(
		ctx,
		chromedp.Nodes(".media-module_connectionTableWrapper_KM74X .PaginationTable_table_z0qIF tbody > tr", &rowNodes, chromedp.ByQueryAll, chromedp.FromNode(currNode), chromedp.AtLeast(0)),
	); err != nil {
		return
	}

	for i := 0; i < len(rowNodes); i++ {
		if err := chromedp.Run(
			ctx,
			chromedp.Text("td:nth-child(1) .PaginationTable_cell_AhJWd b", &pid, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(2) .PaginationTable_cell_AhJWd", &process, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(3) .PaginationTable_cell_AhJWd", &ip, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(4) .PaginationTable_cell_AhJWd", &domain, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(5) .PaginationTable_cell_AhJWd", &asn, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(6) .PaginationTable_cell_AhJWd", &cn, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(7) .PaginationTable_cell_AhJWd span span", &reputation, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
		); err != nil {
			continue
		}

		tcpOrudpConnection := models.TCPOrUDPConnection{
			PID:        pid,
			Process:    process,
			IP:         ip,
			Domain:     domain,
			ASN:        asn,
			CN:         cn,
			Reputation: reputation,
		}

		(*tcpOrudpConnections) = append((*tcpOrudpConnections), tcpOrudpConnection)
	}
}

func scrapeNetworkActivityDNSRequestsPage(ctx context.Context, currNode *cdp.Node, dnsRequests *models.DNSRequests) {
	var (
		rowNodes           []*cdp.Node
		domain, reputation string
		ipNodes            []*cdp.Node
		ips                models.IPs
	)

	if err := chromedp.Run(
		ctx,
		chromedp.Nodes(".media-module_dnsTableWrapper_EEhP2 .PaginationTable_table_z0qIF tbody > tr", &rowNodes, chromedp.ByQueryAll, chromedp.FromNode(currNode), chromedp.AtLeast(0)),
	); err != nil {
		return
	}

	for i := 0; i < len(rowNodes); i++ {
		if err := chromedp.Run(
			ctx,
			chromedp.Text("td:nth-child(1) .PaginationTable_cell_AhJWd b", &domain, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Nodes("td:nth-child(2) .PaginationTable_cell_AhJWd ul li", &ipNodes, chromedp.ByQueryAll, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(3) .PaginationTable_cell_AhJWd span span", &reputation, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
		); err != nil {
			continue
		}

		for _, ipNode := range ipNodes {
			ips = append(ips, ipNode.Children[0].NodeValue)
		}

		dnsRequest := models.DNSRequest{
			Domain:     domain,
			IPs:        ips,
			Reputation: reputation,
		}

		(*dnsRequests) = append((*dnsRequests), dnsRequest)
		ips = models.IPs{}
	}
}

func scrapeNetworkThreatConnectionsPage(ctx context.Context, currNode *cdp.Node, threatConnections *models.ThreatConnections) {
	var (
		rowNodes                     []*cdp.Node
		pid, process, class, message string
	)

	if err := chromedp.Run(
		ctx,
		chromedp.Nodes(".media-module_threatsTableWrapper_Zi8ci .PaginationTable_table_z0qIF tbody > tr", &rowNodes, chromedp.ByQueryAll, chromedp.FromNode(currNode), chromedp.AtLeast(0)),
	); err != nil {
		return
	}

	for i := 0; i < len(rowNodes); i++ {
		if err := chromedp.Run(
			ctx,
			chromedp.Text("td:nth-child(1) .PaginationTable_cell_AhJWd b", &pid, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(2) .PaginationTable_cell_AhJWd", &process, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(3) .PaginationTable_cell_AhJWd span span", &class, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(4) .PaginationTable_cell_AhJWd", &message, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
		); err != nil {
			continue
		}

		threatConnection := models.ThreatConnection{
			PID:     pid,
			Process: process,
			Class:   class,
			Message: message,
		}

		(*threatConnections) = append((*threatConnections), threatConnection)
	}
}

func scrapeDebugOutputStringsNameValueText(ctx context.Context, debugOutputStrings *models.DebugOutputStrings) {
	pageCount := getPageCountTable(
		ctx,
		nil,
		".DebugOutputStrings_ctn_y9cac .DebugOutputStrings_table_duoML .media-module_debugTableWrapper_WeaEB tbody tr:last-child td .PaginationTable_cell_AhJWd .PaginationTable_controls_bHSjz div .Pagination_containerClass_vLdBm li:nth-last-child(3) button",
	)
	scrapeDebugOutputStringsPage(ctx, debugOutputStrings)
	for i := 1; i < pageCount; i++ {
		if err := chromedp.Run(
			ctx,
			chromedp.Click(
				".DebugOutputStrings_ctn_y9cac .DebugOutputStrings_table_duoML .media-module_debugTableWrapper_WeaEB tbody tr:last-child td .PaginationTable_cell_AhJWd .PaginationTable_controls_bHSjz div .Pagination_containerClass_vLdBm li:nth-last-child(2) button",
				chromedp.ByQuery,
				chromedp.NodeVisible,
			),
		); err != nil {
			break
		}
		scrapeDebugOutputStringsPage(ctx, debugOutputStrings)
	}
}

func scrapeDebugOutputStringsPage(ctx context.Context, debugOutputStrings *models.DebugOutputStrings) {
	var (
		rowNodes         []*cdp.Node
		process, message string
	)

	if err := chromedp.Run(
		ctx,
		chromedp.Nodes(".DebugOutputStrings_ctn_y9cac .DebugOutputStrings_table_duoML .media-module_debugTableWrapper_WeaEB .PaginationTable_table_z0qIF tbody > tr:not(:last-child)", &rowNodes, chromedp.ByQueryAll, chromedp.AtLeast(0)),
	); err != nil {
		return
	}

	for i := 0; i < len(rowNodes); i++ {
		if err := chromedp.Run(
			ctx,
			chromedp.Text("td:nth-child(1) .PaginationTable_cell_AhJWd b", &process, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
			chromedp.Text("td:nth-child(2) .PaginationTable_cell_AhJWd", &message, chromedp.ByQuery, chromedp.FromNode(rowNodes[i]), chromedp.AtLeast(0)),
		); err != nil {
			continue
		}

		debugOutputString := models.DebugOutputString{
			Process: process,
			Message: strings.TrimSuffix(message, "\n"),
		}

		(*debugOutputStrings) = append((*debugOutputStrings), debugOutputString)
	}
}

func getPageCountTable(ctx context.Context, currNode *cdp.Node, lastPageQuery string) int {
	var lastPage string
	if err := chromedp.Run(
		ctx,
		chromedp.Text(lastPageQuery, &lastPage, chromedp.ByQuery, chromedp.FromNode(currNode), chromedp.AtLeast(0)),
	); err != nil {
		return 0
	}
	pageCount, err := strconv.Atoi(lastPage)
	if err != nil {
		return 0
	}

	return pageCount
}

var (
	matchSpecialCharacters = regexp.MustCompile(`[^a-zA-Z0-9 ]+`)
	matchNumOfJumps        = regexp.MustCompile(`\(\d+\)`)
)

func toSnakeCase(str string) string {
	snake := matchSpecialCharacters.ReplaceAllString(str, "")
	snake = strings.TrimSpace(snake)
	snake = strings.ReplaceAll(snake, " ", "_")
	return strings.ToLower(snake)
}

func remove0xA0FromString(str string) string {
	return strings.ReplaceAll(str, "\u00a0", " ")
}

func getNumOfJumpsFromKey(key string) int {
	numOfJumpsSlice := matchNumOfJumps.FindStringSubmatch(key)
	if len(numOfJumpsSlice) == 0 {
		return 1
	}
	numOfJumpsSlice[0] = strings.TrimPrefix(numOfJumpsSlice[0], "(")
	numOfJumpsSlice[0] = strings.TrimSuffix(numOfJumpsSlice[0], ")")
	numOfJumps, err := strconv.Atoi(numOfJumpsSlice[0])
	if err != nil {
		return 1
	}
	return numOfJumps
}
