package main

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
)

type Instance struct {
	GuestCpus int64
	MemoryMb  int64
}

func main() {
	ctx := context.Background()

	client, err := google.DefaultClient(ctx, compute.CloudPlatformScope)
	if err != nil {
		log.Fatal(err)
	}

	computeService, err := compute.New(client)
	if err != nil {
		log.Fatal(err)
	}

	projects := []string{
		"intrepid-honor-109516",
		"fresh-8-staging",
		"fresh-8-testing",
		"fresh-8-loadtest",
		"fresh8-uat",
	}

	for _, project := range projects {
		regionsReq := computeService.Regions.List(project)
		if err := regionsReq.Pages(ctx, func(page *compute.RegionList) error {
			for _, region := range page.Items {
				var commitments []Instance
				regionCommitmentReq := computeService.RegionCommitments.List(project, region.Name)
				if err := regionCommitmentReq.Pages(ctx, func(page *compute.CommitmentList) error {
					for _, commitment := range page.Items {
						if commitment.Status != "ACTIVE" {
							continue
						}

						inst, err := getResourceValue(commitment.Resources)
						if err != nil {
							return err
						}

						commitments = append(commitments, inst)
					}

					return nil
				}); err != nil {
					log.Fatal(err)
				}

				var inUse []Instance
				for _, zoneName := range region.Zones {
					zoneParts := strings.Split(zoneName, "/zones/")
					zoneName = zoneParts[1]
					zoneReq := computeService.Zones.Get(project, zoneName)
					zone, err := zoneReq.Do()
					if err != nil {
						log.Fatal(err)
					}

					var instanceTypes []string
					instanceCount := 0
					instancesReq := computeService.Instances.List(project, zone.Name)
					if err := instancesReq.Pages(ctx, func(page *compute.InstanceList) error {
						for _, instance := range page.Items {
							if instance.Status != "RUNNING" {
								continue
							}

							instanceTypeParts := strings.Split(instance.MachineType, "/machineTypes/")
							instanceTypes = append(instanceTypes, instanceTypeParts[1])

							instanceCount = instanceCount + 1
						}

						return nil
					}); err != nil {
						log.Fatal(err)
					}

					machineTypes := make(map[string]*compute.MachineType)

					if instanceCount > 0 {
						machineTypeReq := computeService.MachineTypes.List(project, zone.Name)
						if err := machineTypeReq.Pages(ctx, func(page *compute.MachineTypeList) error {
							for _, machineType := range page.Items {
								machineTypes[machineType.Name] = machineType
							}
							return nil
						}); err != nil {
							log.Fatal(err)
						}
					}

					for _, instanceType := range instanceTypes {
						if machineTypes[instanceType] != nil {
							instanceMachineType := machineTypes[instanceType]

							inUse = append(inUse, Instance{
								GuestCpus: instanceMachineType.GuestCpus,
								MemoryMb:  instanceMachineType.MemoryMb,
							})
						} else {
							instanceTypeParts := strings.Split(instanceType, "-")
							if len(instanceTypeParts) != 3 {
								log.Fatal(instanceType)
							}

							guestCpus, err := strconv.Atoi(instanceTypeParts[1])
							if err != nil {
								log.Fatal(err)
							}

							memoryMb, err := strconv.Atoi(instanceTypeParts[2])
							if err != nil {
								log.Fatal(err)
							}

							inUse = append(inUse, Instance{
								GuestCpus: int64(guestCpus),
								MemoryMb:  int64(memoryMb),
							})
						}
					}
				}

				for _, usage := range inUse {
					for key, commitment := range commitments {
						if usage.MemoryMb == commitment.MemoryMb && usage.GuestCpus == commitment.GuestCpus {
							commitments = append(commitments[:key], commitments[key+1:]...)
							break
						}
					}
				}

				if len(commitments) > 0 {
					log.Println(project, region.Name, len(commitments))
					log.Println(commitments)
				}
			}

			return nil
		}); err != nil {
			log.Fatal(err)
		}
	}

}

func getResourceValue(rcs []*compute.ResourceCommitment) (Instance, error) {
	var inst Instance
	for _, rc := range rcs {
		switch rc.Type {
		case "MEMORY":
			inst.MemoryMb = rc.Amount
		case "VCPU":
			inst.GuestCpus = rc.Amount
		default:
			return inst, errors.New("unspecified resource type")
		}
	}

	return inst, nil
}
