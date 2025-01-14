package usecases

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/context-box/server/utils"
)

// SaveConfigOperator saves config to MongoDB after receiving it from the Operator microservice
func (u *Usecases) SaveConfigOperator(request *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
	// Input specs can be changed by 2 entities - by Autoscaler or by User. There is a possibility that both of them can do it
	// at the same time. Thus, we need to lock the config while one entity updates it in the database, so the other entity does
	// not overwrite it.
	u.configChangeMutex.Lock()
	defer u.configChangeMutex.Unlock()

	newConfig := request.GetConfig()
	log.Info().Msgf("Saving config %s from claudie-operator", newConfig.Name)

	newConfig.MsChecksum = utils.CalculateChecksum(newConfig.Manifest)

	// Check if config with this name already exists in MongoDB
	oldConfig, err := u.DB.GetConfig(newConfig.GetName(), pb.IdType_NAME)
	if err == nil {
		if string(oldConfig.MsChecksum) != string(newConfig.MsChecksum) {
			oldConfig.Manifest = newConfig.Manifest
			oldConfig.MsChecksum = newConfig.MsChecksum
			oldConfig.SchedulerTTL = 0
			oldConfig.BuilderTTL = 0
			// clear error states (if any), to push the changed config into the workflow again.
			for cluster, wf := range oldConfig.State {
				if wf.Status == pb.Workflow_ERROR {
					oldConfig.State[cluster] = &pb.Workflow{}
				}
			}
		}
		newConfig = oldConfig
	}

	if err = u.DB.SaveConfig(newConfig); err != nil {
		return nil, fmt.Errorf("error while saving config %s in MongoDB: %w", newConfig.Name, err)
	}

	log.Info().Msgf("Config %s successfully saved from claudie-operator", newConfig.Name)

	return &pb.SaveConfigResponse{Config: newConfig}, nil
}
