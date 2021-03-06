package reporting

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	metering "github.com/operator-framework/operator-metering/pkg/apis/metering/v1alpha1"
	meteringClient "github.com/operator-framework/operator-metering/pkg/generated/clientset/versioned/typed/metering/v1alpha1"
	meteringListers "github.com/operator-framework/operator-metering/pkg/generated/listers/metering/v1alpha1"
)

const maxDepth = 100

type ReportGenerationQueryDependencies struct {
	ReportGenerationQueries        []*metering.ReportGenerationQuery
	DynamicReportGenerationQueries []*metering.ReportGenerationQuery
	ReportDataSources              []*metering.ReportDataSource
}

func ValidateGenerationQueryDependenciesStatus(depsStatus *GenerationQueryDependenciesStatus) (*ReportGenerationQueryDependencies, error) {
	// if the specified ReportGenerationQuery depends on other non-dynamic
	// ReportGenerationQueries, but they have their view disabled, then it's an
	// invalid configuration.
	var queriesViewDisabled, uninitializedQueries, uninitializedDataSources []string
	for _, query := range depsStatus.UninitializedReportGenerationQueries {
		if query.Spec.View.Disabled {
			queriesViewDisabled = append(queriesViewDisabled, query.Name)
		} else if query.ViewName == "" {
			uninitializedQueries = append(uninitializedQueries, query.Name)
		}
	}
	for _, ds := range depsStatus.UninitializedReportDataSources {
		uninitializedDataSources = append(uninitializedDataSources, ds.Name)
	}
	if len(queriesViewDisabled) != 0 {
		return nil, fmt.Errorf("invalid ReportGenerationQuery, references ReportGenerationQueries with spec.view.disabled=true: %s", strings.Join(queriesViewDisabled, ", "))
	}
	if len(uninitializedDataSources) != 0 {
		return nil, fmt.Errorf("ReportGenerationQuery has uninitialized ReportDataSource dependencies: %s", strings.Join(uninitializedDataSources, ", "))
	}
	if len(uninitializedQueries) != 0 {
		return nil, fmt.Errorf("ReportGenerationQuery has uninitialized ReportGenerationQuery dependencies: %s", strings.Join(uninitializedQueries, ", "))
	}

	return &ReportGenerationQueryDependencies{
		ReportGenerationQueries:        depsStatus.InitializedReportGenerationQueries,
		DynamicReportGenerationQueries: depsStatus.InitializedDynamicReportGenerationQueries,
		ReportDataSources:              depsStatus.InitializedReportDataSources,
	}, nil
}

type GenerationQueryDependenciesStatus struct {
	UninitializedReportGenerationQueries      []*metering.ReportGenerationQuery
	InitializedReportGenerationQueries        []*metering.ReportGenerationQuery
	InitializedDynamicReportGenerationQueries []*metering.ReportGenerationQuery

	UninitializedReportDataSources []*metering.ReportDataSource
	InitializedReportDataSources   []*metering.ReportDataSource
}

func GetGenerationQueryDependenciesStatus(queryGetter reportGenerationQueryGetter, dataSourceGetter reportDataSourceGetter, generationQuery *metering.ReportGenerationQuery) (*GenerationQueryDependenciesStatus, error) {
	// Validate ReportGenerationQuery's that should be views
	dependentQueriesStatus, err := GetDependentGenerationQueries(queryGetter, generationQuery)
	if err != nil {
		return nil, err
	}

	dataSources, err := GetDependentDataSources(dataSourceGetter, generationQuery)
	if err != nil {
		return nil, err
	}

	var uninitializedDataSources, initializedDataSources []*metering.ReportDataSource
	for _, dataSource := range dataSources {
		if dataSource.TableName == "" {
			uninitializedDataSources = append(uninitializedDataSources, dataSource)
		} else {
			initializedDataSources = append(initializedDataSources, dataSource)
		}
	}

	var uninitializedQueries, initializedQueries []*metering.ReportGenerationQuery
	for _, query := range dependentQueriesStatus.ViewReportGenerationQueries {
		if query.ViewName == "" {
			uninitializedQueries = append(uninitializedQueries, query)
		} else {
			initializedQueries = append(initializedQueries, query)
		}
	}

	return &GenerationQueryDependenciesStatus{
		UninitializedReportGenerationQueries:      uninitializedQueries,
		InitializedReportGenerationQueries:        initializedQueries,
		InitializedDynamicReportGenerationQueries: dependentQueriesStatus.DynamicReportGenerationQueries,
		UninitializedReportDataSources:            uninitializedDataSources,
		InitializedReportDataSources:              initializedDataSources,
	}, nil
}

type GetDependentGenerationQueriesStatus struct {
	ViewReportGenerationQueries    []*metering.ReportGenerationQuery
	DynamicReportGenerationQueries []*metering.ReportGenerationQuery
}

func GetDependentGenerationQueries(queryGetter reportGenerationQueryGetter, generationQuery *metering.ReportGenerationQuery) (*GetDependentGenerationQueriesStatus, error) {
	viewQueries, err := GetDependentViewGenerationQueries(queryGetter, generationQuery)
	if err != nil {
		return nil, err
	}
	dynamicQueries, err := GetDependentDynamicGenerationQueries(queryGetter, generationQuery)
	if err != nil {
		return nil, err
	}
	return &GetDependentGenerationQueriesStatus{
		ViewReportGenerationQueries:    viewQueries,
		DynamicReportGenerationQueries: dynamicQueries,
	}, nil
}

func GetDependentViewGenerationQueries(queryGetter reportGenerationQueryGetter, generationQuery *metering.ReportGenerationQuery) ([]*metering.ReportGenerationQuery, error) {
	viewReportQueriesAccumulator := make(map[string]*metering.ReportGenerationQuery)
	err := GetDependentGenerationQueriesMemoized(queryGetter, generationQuery, 0, maxDepth, viewReportQueriesAccumulator, false)
	if err != nil {
		return nil, err
	}

	viewQueries := make([]*metering.ReportGenerationQuery, 0, len(viewReportQueriesAccumulator))
	for _, query := range viewReportQueriesAccumulator {
		viewQueries = append(viewQueries, query)
	}
	return viewQueries, nil
}

func GetDependentDynamicGenerationQueries(queryGetter reportGenerationQueryGetter, generationQuery *metering.ReportGenerationQuery) ([]*metering.ReportGenerationQuery, error) {
	dynamicReportQueriesAccumulator := make(map[string]*metering.ReportGenerationQuery)
	err := GetDependentGenerationQueriesMemoized(queryGetter, generationQuery, 0, maxDepth, dynamicReportQueriesAccumulator, true)
	if err != nil {
		return nil, err
	}

	dynamicQueries := make([]*metering.ReportGenerationQuery, 0, len(dynamicReportQueriesAccumulator))
	for _, query := range dynamicReportQueriesAccumulator {
		dynamicQueries = append(dynamicQueries, query)
	}
	return dynamicQueries, nil
}

type reportGenerationQueryGetter interface {
	getReportGenerationQuery(namespace, name string) (*metering.ReportGenerationQuery, error)
}

type reportGenerationQueryGetterFunc func(string, string) (*metering.ReportGenerationQuery, error)

func (f reportGenerationQueryGetterFunc) getReportGenerationQuery(namespace, name string) (*metering.ReportGenerationQuery, error) {
	return f(namespace, name)
}

func NewReportGenerationQueryListerGetter(lister meteringListers.ReportGenerationQueryLister) reportGenerationQueryGetter {
	return reportGenerationQueryGetterFunc(func(namespace, name string) (*metering.ReportGenerationQuery, error) {
		return lister.ReportGenerationQueries(namespace).Get(name)
	})
}

func NewReportGenerationQueryClientGetter(getter meteringClient.ReportGenerationQueriesGetter) reportGenerationQueryGetter {
	return reportGenerationQueryGetterFunc(func(namespace, name string) (*metering.ReportGenerationQuery, error) {
		return getter.ReportGenerationQueries(namespace).Get(name, metav1.GetOptions{})
	})
}

func GetDependentGenerationQueriesMemoized(queryGetter reportGenerationQueryGetter, generationQuery *metering.ReportGenerationQuery, depth, maxDepth int, queriesAccumulator map[string]*metering.ReportGenerationQuery, dynamicQueries bool) error {
	if depth >= maxDepth {
		return fmt.Errorf("detected a cycle at depth %d for generationQuery %s", depth, generationQuery.Name)
	}
	var queries []string
	if dynamicQueries {
		queries = generationQuery.Spec.DynamicReportQueries
	} else {
		queries = generationQuery.Spec.ReportQueries
	}
	for _, queryName := range queries {
		if _, exists := queriesAccumulator[queryName]; exists {
			continue
		}
		genQuery, err := queryGetter.getReportGenerationQuery(generationQuery.Namespace, queryName)
		if err != nil {
			return err
		}
		err = GetDependentGenerationQueriesMemoized(queryGetter, genQuery, depth+1, maxDepth, queriesAccumulator, dynamicQueries)
		if err != nil {
			return err
		}
		queriesAccumulator[genQuery.Name] = genQuery
	}
	return nil
}

type reportDataSourceGetter interface {
	getReportDataSource(namespace, name string) (*metering.ReportDataSource, error)
}

type reportDataSourceGetterFunc func(string, string) (*metering.ReportDataSource, error)

func (f reportDataSourceGetterFunc) getReportDataSource(namespace, name string) (*metering.ReportDataSource, error) {
	return f(namespace, name)
}

func NewReportDataSourceListerGetter(lister meteringListers.ReportDataSourceLister) reportDataSourceGetter {
	return reportDataSourceGetterFunc(func(namespace, name string) (*metering.ReportDataSource, error) {
		return lister.ReportDataSources(namespace).Get(name)
	})
}

func NewReportDataSourceClientGetter(getter meteringClient.ReportDataSourcesGetter) reportDataSourceGetter {
	return reportDataSourceGetterFunc(func(namespace, name string) (*metering.ReportDataSource, error) {
		return getter.ReportDataSources(namespace).Get(name, metav1.GetOptions{})
	})
}

func GetDependentDataSources(dataSourceGetter reportDataSourceGetter, generationQuery *metering.ReportGenerationQuery) ([]*metering.ReportDataSource, error) {
	dataSources := make([]*metering.ReportDataSource, len(generationQuery.Spec.DataSources))
	for i, dataSourceName := range generationQuery.Spec.DataSources {
		dataSource, err := dataSourceGetter.getReportDataSource(generationQuery.Namespace, dataSourceName)
		if err != nil {
			return nil, err
		}
		dataSources[i] = dataSource
	}
	return dataSources, nil
}
