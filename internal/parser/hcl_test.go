func TestParseFile_EKSLikeDev_ReferenceExtraction(t *testing.T) {
	path := filepath.Join(fixturesDir(t), "eks-like", "environments", "dev", "main.tf")
	result := ParseFile(path, "dev")
	if result.Error != nil {
		t.Fatalf("unexpected parse error: %v", result.Error)
	}

	var apiWorker *ResourceBlock
	for i := range result.Resources {
		if result.Resources[i].Type == "aws_ecs_service" && result.Resources[i].Name == "api_worker" {
			apiWorker = &result.Resources[i]
		}
	}
	if apiWorker == nil {
		t.Fatal("aws_ecs_service.api_worker not found")
	}

	clusterAttr, ok := apiWorker.Attributes["cluster"]
	if !ok {
		t.Fatal("expected `cluster` attribute on api_worker")
	}
	if clusterAttr.Reference == nil {
		t.Fatal("expected a populated Reference for a direct_ref attribute")
	}
	want := ResourceReference{TargetType: "aws_ecs_cluster", TargetName: "main", TargetAttribute: "id"}
	if *clusterAttr.Reference != want {
		t.Errorf("got %+v, want %+v", *clusterAttr.Reference, want)
	}
}

func TestParseFile_EKSLikeStaging_HardcodedString_NoReference(t *testing.T) {
	path := filepath.Join(fixturesDir(t), "eks-like", "environments", "staging", "main.tf")
	result := ParseFile(path, "staging")
	if result.Error != nil {
		t.Fatalf("unexpected parse error: %v", result.Error)
	}

	var apiWorker *ResourceBlock
	for i := range result.Resources {
		if result.Resources[i].Type == "aws_ecs_service" && result.Resources[i].Name == "api_worker" {
			apiWorker = &result.Resources[i]
		}
	}
	if apiWorker == nil {
		t.Fatal("aws_ecs_service.api_worker not found in staging fixture")
	}

	clusterAttr, ok := apiWorker.Attributes["cluster"]
	if !ok {
		t.Fatal("expected `cluster` attribute")
	}
	if clusterAttr.Reference != nil {
		t.Errorf("hardcoded string attribute should have nil Reference, got %+v", *clusterAttr.Reference)
	}
}
