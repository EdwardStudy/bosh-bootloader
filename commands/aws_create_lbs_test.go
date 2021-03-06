package commands_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/cloudfoundry/bosh-bootloader/aws/iam"
	"github.com/cloudfoundry/bosh-bootloader/commands"
	"github.com/cloudfoundry/bosh-bootloader/fakes"
	"github.com/cloudfoundry/bosh-bootloader/storage"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("AWS Create LBs", func() {
	Describe("Execute", func() {
		var (
			command                   commands.AWSCreateLBs
			certificateManager        *fakes.CertificateManager
			infrastructureManager     *fakes.InfrastructureManager
			terraformManager          *fakes.TerraformManager
			availabilityZoneRetriever *fakes.AvailabilityZoneRetriever
			credentialValidator       *fakes.CredentialValidator
			logger                    *fakes.Logger
			cloudConfigManager        *fakes.CloudConfigManager
			certificateValidator      *fakes.CertificateValidator
			guidGenerator             *fakes.GuidGenerator
			stateStore                *fakes.StateStore
			environmentValidator      *fakes.EnvironmentValidator
			incomingState             storage.State
		)

		BeforeEach(func() {
			certificateManager = &fakes.CertificateManager{}
			infrastructureManager = &fakes.InfrastructureManager{}
			terraformManager = &fakes.TerraformManager{}
			availabilityZoneRetriever = &fakes.AvailabilityZoneRetriever{}
			credentialValidator = &fakes.CredentialValidator{}
			logger = &fakes.Logger{}
			cloudConfigManager = &fakes.CloudConfigManager{}
			certificateValidator = &fakes.CertificateValidator{}
			guidGenerator = &fakes.GuidGenerator{}
			stateStore = &fakes.StateStore{}
			environmentValidator = &fakes.EnvironmentValidator{}

			infrastructureManager.ExistsCall.Returns.Exists = true

			guidGenerator.GenerateCall.Returns.Output = "abcd"

			incomingState = storage.State{
				Stack: storage.Stack{
					Name:   "some-stack",
					BOSHAZ: "some-bosh-az",
				},
				AWS: storage.AWS{
					AccessKeyID:     "some-access-key-id",
					SecretAccessKey: "some-secret-access-key",
					Region:          "some-region",
				},
				KeyPair: storage.KeyPair{
					Name: "some-key-pair",
				},
				BOSH: storage.BOSH{
					DirectorAddress:  "some-director-address",
					DirectorUsername: "some-director-username",
					DirectorPassword: "some-director-password",
				},
				EnvID: "some-env-id-timestamp",
			}

			command = commands.NewAWSCreateLBs(logger, credentialValidator, certificateManager, infrastructureManager,
				availabilityZoneRetriever, cloudConfigManager, certificateValidator, guidGenerator,
				stateStore, terraformManager, environmentValidator)
		})

		It("returns an error if credential validator fails", func() {
			credentialValidator.ValidateCall.Returns.Error = errors.New("failed to validate aws credentials")
			err := command.Execute(commands.AWSCreateLBsConfig{}, storage.State{})
			Expect(err).To(MatchError("failed to validate aws credentials"))
		})

		It("uploads a cert and key", func() {
			err := command.Execute(commands.AWSCreateLBsConfig{
				LBType:   "concourse",
				CertPath: "temp/some-cert.crt",
				KeyPath:  "temp/some-key.key",
			}, incomingState)
			Expect(err).NotTo(HaveOccurred())

			Expect(certificateManager.CreateCall.Receives.Certificate).To(Equal("temp/some-cert.crt"))
			Expect(certificateManager.CreateCall.Receives.PrivateKey).To(Equal("temp/some-key.key"))
			Expect(certificateManager.CreateCall.Receives.CertificateName).To(Equal("concourse-elb-cert-abcd-some-env-id-timestamp"))
			Expect(logger.StepCall.Messages).To(ContainElement("uploading certificate"))

		})

		It("uploads a cert and key with chain", func() {
			err := command.Execute(commands.AWSCreateLBsConfig{
				LBType:    "concourse",
				CertPath:  "temp/some-cert.crt",
				KeyPath:   "temp/some-key.key",
				ChainPath: "temp/some-chain.crt",
			}, incomingState)
			Expect(err).NotTo(HaveOccurred())

			Expect(certificateManager.CreateCall.Receives.Chain).To(Equal("temp/some-chain.crt"))

			Expect(certificateValidator.ValidateCall.Receives.Command).To(Equal("create-lbs"))
			Expect(certificateValidator.ValidateCall.Receives.CertificatePath).To(Equal("temp/some-cert.crt"))
			Expect(certificateValidator.ValidateCall.Receives.KeyPath).To(Equal("temp/some-key.key"))
			Expect(certificateValidator.ValidateCall.Receives.ChainPath).To(Equal("temp/some-chain.crt"))
		})

		Context("when cloudformation was used to create infrastructure", func() {
			It("creates a load balancer in cloudformation with certificate", func() {
				availabilityZoneRetriever.RetrieveCall.Returns.AZs = []string{"a", "b", "c"}
				certificateManager.DescribeCall.Returns.Certificate = iam.Certificate{
					ARN: "some-certificate-arn",
				}

				err := command.Execute(commands.AWSCreateLBsConfig{
					LBType:   "concourse",
					CertPath: "temp/some-cert.crt",
					KeyPath:  "temp/some-key.key",
				}, incomingState)
				Expect(err).NotTo(HaveOccurred())

				Expect(availabilityZoneRetriever.RetrieveCall.Receives.Region).To(Equal("some-region"))

				Expect(certificateManager.DescribeCall.Receives.CertificateName).To(Equal("concourse-elb-cert-abcd-some-env-id-timestamp"))

				Expect(infrastructureManager.UpdateCall.Receives.KeyPairName).To(Equal("some-key-pair"))
				Expect(infrastructureManager.UpdateCall.Receives.AZs).To(Equal([]string{"a", "b", "c"}))
				Expect(infrastructureManager.UpdateCall.Receives.StackName).To(Equal("some-stack"))
				Expect(infrastructureManager.UpdateCall.Receives.LBType).To(Equal("concourse"))
				Expect(infrastructureManager.UpdateCall.Receives.LBCertificateARN).To(Equal("some-certificate-arn"))
				Expect(infrastructureManager.UpdateCall.Receives.EnvID).To(Equal("some-env-id-timestamp"))
				Expect(infrastructureManager.UpdateCall.Receives.BOSHAZ).To(Equal("some-bosh-az"))
			})
		})

		Context("when terraform was used to create infrastructure", func() {
			var (
				statePassedToTerraform     storage.State
				stateReturnedFromTerraform storage.State

				certPath  string
				keyPath   string
				chainPath string
			)

			BeforeEach(func() {
				incomingState = storage.State{
					AWS: storage.AWS{
						AccessKeyID:     "some-access-key-id",
						SecretAccessKey: "some-secret-access-key",
						Region:          "some-region",
					},
					KeyPair: storage.KeyPair{
						Name: "some-key-pair",
					},
					TFState: "some-tf-state",
					BOSH: storage.BOSH{
						DirectorAddress:  "some-director-address",
						DirectorUsername: "some-director-username",
						DirectorPassword: "some-director-password",
					},
					EnvID: "some-env-id-timestamp",
				}

				availabilityZoneRetriever.RetrieveCall.Returns.AZs = []string{"a", "b", "c"}
				certificateManager.DescribeCall.Returns.Certificate = iam.Certificate{
					ARN: "some-certificate-arn",
				}

				tempCertFile, err := ioutil.TempFile("", "cert")
				Expect(err).NotTo(HaveOccurred())

				certificate := "some-cert"
				certPath = tempCertFile.Name()
				err = ioutil.WriteFile(certPath, []byte(certificate), os.ModePerm)
				Expect(err).NotTo(HaveOccurred())

				tempKeyFile, err := ioutil.TempFile("", "key")
				Expect(err).NotTo(HaveOccurred())

				key := "some-key"
				keyPath = tempKeyFile.Name()
				err = ioutil.WriteFile(keyPath, []byte(key), os.ModePerm)
				Expect(err).NotTo(HaveOccurred())

				tempChainFile, err := ioutil.TempFile("", "chain")
				Expect(err).NotTo(HaveOccurred())

				chain := "some-chain"
				chainPath = tempChainFile.Name()
				err = ioutil.WriteFile(chainPath, []byte(chain), os.ModePerm)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when lb type desired is cf", func() {
				BeforeEach(func() {
					statePassedToTerraform = incomingState
					statePassedToTerraform.LB = storage.LB{
						Type: "cf",
						Cert: "some-cert",
						Key:  "some-key",
					}

					stateReturnedFromTerraform = statePassedToTerraform
					stateReturnedFromTerraform.TFState = "some-updated-tf-state"
					terraformManager.ApplyCall.Returns.BBLState = stateReturnedFromTerraform
				})

				It("creates a load balancer with certificate using terraform", func() {
					err := command.Execute(commands.AWSCreateLBsConfig{
						LBType:   "cf",
						CertPath: certPath,
						KeyPath:  keyPath,
					}, incomingState)
					Expect(err).NotTo(HaveOccurred())

					Expect(certificateValidator.ValidateCall.CallCount).To(Equal(0))
					Expect(infrastructureManager.UpdateCall.CallCount).To(Equal(0))
					Expect(certificateManager.CreateCall.CallCount).To(Equal(0))
					Expect(terraformManager.ApplyCall.Receives.BBLState).To(Equal(statePassedToTerraform))
					Expect(stateStore.SetCall.Receives[0].State).To(Equal(stateReturnedFromTerraform))
				})

				Context("when the optional chain is provided", func() {
					BeforeEach(func() {
						statePassedToTerraform.LB.Chain = "some-chain"

						stateReturnedFromTerraform = statePassedToTerraform
						stateReturnedFromTerraform.TFState = "some-updated-tf-state"
						terraformManager.ApplyCall.Returns.BBLState = stateReturnedFromTerraform
					})

					It("creates a load balancer with certificate using terraform", func() {
						err := command.Execute(commands.AWSCreateLBsConfig{
							LBType:    "cf",
							CertPath:  certPath,
							KeyPath:   keyPath,
							ChainPath: chainPath,
						}, incomingState)
						Expect(err).NotTo(HaveOccurred())

						Expect(terraformManager.ApplyCall.Receives.BBLState).To(Equal(statePassedToTerraform))
						Expect(stateStore.SetCall.Receives[0].State).To(Equal(stateReturnedFromTerraform))
					})
				})

				Context("when a domain is provided", func() {
					BeforeEach(func() {
						statePassedToTerraform.LB = storage.LB{
							Type:   "cf",
							Cert:   "some-cert",
							Key:    "some-key",
							Domain: "some-domain",
						}

						stateReturnedFromTerraform = statePassedToTerraform
						stateReturnedFromTerraform.TFState = "some-updated-tf-state"
						terraformManager.ApplyCall.Returns.BBLState = stateReturnedFromTerraform
					})

					It("creates dns records for provided domain", func() {
						err := command.Execute(commands.AWSCreateLBsConfig{
							LBType:   "cf",
							CertPath: certPath,
							KeyPath:  keyPath,
							Domain:   "some-domain",
						}, incomingState)
						Expect(err).NotTo(HaveOccurred())

						Expect(terraformManager.ApplyCall.Receives.BBLState).To(Equal(statePassedToTerraform))
						Expect(stateStore.SetCall.Receives[0].State).To(Equal(stateReturnedFromTerraform))
					})
				})

				Context("when a domain exists", func() {
					BeforeEach(func() {
						incomingState.LB = storage.LB{
							Type:   "cf",
							Cert:   "some-cert",
							Key:    "some-key",
							Domain: "some-domain",
						}
						statePassedToTerraform = incomingState

						stateReturnedFromTerraform = statePassedToTerraform
						stateReturnedFromTerraform.TFState = "some-updated-tf-state"
						terraformManager.ApplyCall.Returns.BBLState = stateReturnedFromTerraform
					})

					It("does not change domain", func() {
						err := command.Execute(commands.AWSCreateLBsConfig{
							LBType:   "cf",
							CertPath: certPath,
							KeyPath:  keyPath,
						}, incomingState)
						Expect(err).NotTo(HaveOccurred())

						Expect(terraformManager.ApplyCall.Receives.BBLState).To(Equal(statePassedToTerraform))
						Expect(stateStore.SetCall.Receives[0].State).To(Equal(stateReturnedFromTerraform))
					})
				})
			})

			Context("when lb type desired is concourse", func() {
				BeforeEach(func() {
					statePassedToTerraform = incomingState
					statePassedToTerraform.LB = storage.LB{
						Type: "concourse",
						Cert: "some-cert",
						Key:  "some-key",
					}

					stateReturnedFromTerraform = statePassedToTerraform
					stateReturnedFromTerraform.TFState = "some-updated-tf-state"
					terraformManager.ApplyCall.Returns.BBLState = stateReturnedFromTerraform
				})

				It("creates a load balancer with certificate using terraform", func() {
					err := command.Execute(commands.AWSCreateLBsConfig{
						LBType:   "concourse",
						CertPath: certPath,
						KeyPath:  keyPath,
					}, incomingState)
					Expect(err).NotTo(HaveOccurred())

					Expect(certificateValidator.ValidateCall.CallCount).To(Equal(0))
					Expect(infrastructureManager.UpdateCall.CallCount).To(Equal(0))
					Expect(certificateManager.CreateCall.CallCount).To(Equal(0))
					Expect(terraformManager.ApplyCall.Receives.BBLState).To(Equal(statePassedToTerraform))
					Expect(stateStore.SetCall.Receives[0].State).To(Equal(stateReturnedFromTerraform))
				})

				Context("when optional chain is provided", func() {
					BeforeEach(func() {
						statePassedToTerraform.LB.Chain = "some-chain"

						stateReturnedFromTerraform = statePassedToTerraform
						stateReturnedFromTerraform.TFState = "some-updated-tf-state"
						terraformManager.ApplyCall.Returns.BBLState = stateReturnedFromTerraform
					})

					It("creates a load balancer with certificate using terraform", func() {
						err := command.Execute(commands.AWSCreateLBsConfig{
							LBType:    "concourse",
							CertPath:  certPath,
							KeyPath:   keyPath,
							ChainPath: chainPath,
						}, incomingState)
						Expect(err).NotTo(HaveOccurred())

						Expect(terraformManager.ApplyCall.Receives.BBLState).To(Equal(statePassedToTerraform))
						Expect(stateStore.SetCall.Receives[0].State).To(Equal(stateReturnedFromTerraform))
					})
				})
			})
		})

		It("names the loadbalancer without EnvID when EnvID is not set", func() {
			incomingState.EnvID = ""

			availabilityZoneRetriever.RetrieveCall.Returns.AZs = []string{"a", "b", "c"}
			certificateManager.DescribeCall.Returns.Certificate = iam.Certificate{
				ARN: "some-certificate-arn",
			}

			err := command.Execute(commands.AWSCreateLBsConfig{
				LBType:   "concourse",
				CertPath: "temp/some-cert.crt",
				KeyPath:  "temp/some-key.key",
			}, incomingState)
			Expect(err).NotTo(HaveOccurred())

			Expect(certificateManager.DescribeCall.Receives.CertificateName).To(Equal("concourse-elb-cert-abcd"))

		})

		Context("when the bbl environment has a BOSH director", func() {
			It("updates the cloud config with a state that has lb type", func() {
				err := command.Execute(commands.AWSCreateLBsConfig{
					LBType:   "concourse",
					CertPath: "temp/some-cert.crt",
					KeyPath:  "temp/some-key.key",
				}, incomingState)
				Expect(err).NotTo(HaveOccurred())

				Expect(cloudConfigManager.UpdateCall.Receives.State.Stack.LBType).To(Equal("concourse"))
			})
		})

		Context("when the bbl environment does not have a BOSH director", func() {
			It("does not create a BOSH client", func() {
				incomingState = storage.State{
					NoDirector: true,
					Stack: storage.Stack{
						Name:   "some-stack",
						BOSHAZ: "some-bosh-az",
					},
					AWS: storage.AWS{
						AccessKeyID:     "some-access-key-id",
						SecretAccessKey: "some-secret-access-key",
						Region:          "some-region",
					},
					KeyPair: storage.KeyPair{
						Name: "some-key-pair",
					},
					EnvID: "some-env-id-timestamp",
				}
				err := command.Execute(commands.AWSCreateLBsConfig{
					LBType:   "concourse",
					CertPath: "temp/some-cert.crt",
					KeyPath:  "temp/some-key.key",
				}, incomingState)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not call cloudConfigManager", func() {
				incomingState = storage.State{
					NoDirector: true,
					Stack: storage.Stack{
						Name:   "some-stack",
						BOSHAZ: "some-bosh-az",
					},
					AWS: storage.AWS{
						AccessKeyID:     "some-access-key-id",
						SecretAccessKey: "some-secret-access-key",
						Region:          "some-region",
					},
					KeyPair: storage.KeyPair{
						Name: "some-key-pair",
					},
					EnvID: "some-env-id-timestamp",
				}
				err := command.Execute(commands.AWSCreateLBsConfig{
					LBType:   "concourse",
					CertPath: "temp/some-cert.crt",
					KeyPath:  "temp/some-key.key",
				}, incomingState)
				Expect(err).NotTo(HaveOccurred())

				Expect(cloudConfigManager.UpdateCall.CallCount).To(Equal(0))
			})
		})

		Context("when --skip-if-exists is provided", func() {
			It("no-ops when lb exists", func() {
				incomingState.Stack.LBType = "cf"
				err := command.Execute(commands.AWSCreateLBsConfig{
					LBType:       "concourse",
					CertPath:     "temp/some-cert.crt",
					KeyPath:      "temp/some-key.key",
					SkipIfExists: true,
				}, incomingState)
				Expect(err).NotTo(HaveOccurred())

				Expect(infrastructureManager.UpdateCall.CallCount).To(Equal(0))
				Expect(certificateManager.CreateCall.CallCount).To(Equal(0))

				Expect(logger.PrintlnCall.Receives.Message).To(Equal(`lb type "cf" exists, skipping...`))
			})

			DescribeTable("creates the lb if the lb does not exist",
				func(currentLBType string) {
					incomingState.Stack.LBType = currentLBType
					err := command.Execute(commands.AWSCreateLBsConfig{
						LBType:       "concourse",
						CertPath:     "temp/some-cert.crt",
						KeyPath:      "temp/some-key.key",
						SkipIfExists: true,
					}, incomingState)
					Expect(err).NotTo(HaveOccurred())

					Expect(infrastructureManager.UpdateCall.CallCount).To(Equal(1))
					Expect(certificateManager.CreateCall.CallCount).To(Equal(1))
				},
				Entry("when the current lb-type is 'none'", "none"),
				Entry("when the current lb-type is ''", ""),
			)
		})

		Context("invalid lb type", func() {
			It("returns an error", func() {
				err := command.Execute(commands.AWSCreateLBsConfig{
					LBType:   "some-invalid-lb",
					CertPath: "temp/some-cert.crt",
					KeyPath:  "temp/some-key.key",
				}, incomingState)
				Expect(err).To(MatchError("\"some-invalid-lb\" is not a valid lb type, valid lb types are: concourse and cf"))
			})

			It("returns a helpful error when no lb type is provided", func() {
				err := command.Execute(commands.AWSCreateLBsConfig{
					LBType:   "",
					CertPath: "temp/some-cert.crt",
					KeyPath:  "temp/some-key.key",
				}, incomingState)
				Expect(err).To(MatchError("--type is a required flag"))
			})
		})

		It("returns an error when the environment validator fails", func() {
			environmentValidator.ValidateCall.Returns.Error = errors.New("environment not found")

			err := command.Execute(commands.AWSCreateLBsConfig{
				LBType:   "concourse",
				CertPath: "temp/some-cert.crt",
				KeyPath:  "temp/some-key.key",
			}, incomingState)

			Expect(environmentValidator.ValidateCall.Receives.State).To(Equal(incomingState))
			Expect(err).To(MatchError("environment not found"))
		})

		Context("state manipulation", func() {
			Context("when the env id does not exist", func() {
				It("saves state with new certificate name and lb type", func() {
					err := command.Execute(commands.AWSCreateLBsConfig{
						LBType:   "concourse",
						CertPath: "temp/some-cert.crt",
						KeyPath:  "temp/some-key.key",
					}, storage.State{})
					Expect(err).NotTo(HaveOccurred())

					Expect(stateStore.SetCall.CallCount).To(Equal(1))
					state := stateStore.SetCall.Receives[0].State
					Expect(state.Stack.CertificateName).To(Equal("concourse-elb-cert-abcd"))
					Expect(state.Stack.LBType).To(Equal("concourse"))
				})
			})

			Context("when the env id exists", func() {
				It("saves state with new certificate name and lb type", func() {
					err := command.Execute(commands.AWSCreateLBsConfig{
						LBType:   "concourse",
						CertPath: "temp/some-cert.crt",
						KeyPath:  "temp/some-key.key",
					}, storage.State{
						EnvID: "some-env-id-timestamp",
					})
					Expect(err).NotTo(HaveOccurred())

					Expect(stateStore.SetCall.CallCount).To(Equal(1))
					state := stateStore.SetCall.Receives[0].State
					Expect(state.Stack.CertificateName).To(Equal("concourse-elb-cert-abcd-some-env-id-timestamp"))
					Expect(state.Stack.LBType).To(Equal("concourse"))
				})
			})
		})

		Context("required args", func() {
			It("returns an error when certificate validator fails for cert and key", func() {
				certificateValidator.ValidateCall.Returns.Error = errors.New("failed to validate")
				err := command.Execute(commands.AWSCreateLBsConfig{
					LBType:   "concourse",
					CertPath: "/path/to/cert",
					KeyPath:  "/path/to/key",
				}, storage.State{})

				Expect(err).To(MatchError("failed to validate"))
				Expect(certificateValidator.ValidateCall.Receives.Command).To(Equal("create-lbs"))
				Expect(certificateValidator.ValidateCall.Receives.CertificatePath).To(Equal("/path/to/cert"))
				Expect(certificateValidator.ValidateCall.Receives.KeyPath).To(Equal("/path/to/key"))
				Expect(certificateValidator.ValidateCall.Receives.ChainPath).To(Equal(""))

				Expect(certificateManager.CreateCall.CallCount).To(Equal(0))
			})
		})

		Context("failure cases", func() {
			DescribeTable("returns an error when an lb already exists",
				func(newLbType, oldLbType string) {
					err := command.Execute(commands.AWSCreateLBsConfig{
						LBType:   "concourse",
						CertPath: "/path/to/cert",
						KeyPath:  "/path/to/key",
					}, storage.State{
						Stack: storage.Stack{
							LBType: oldLbType,
						},
					})
					Expect(err).To(MatchError(fmt.Sprintf("bbl already has a %s load balancer attached, please remove the previous load balancer before attaching a new one", oldLbType)))
				},
				Entry("when the previous lb type is concourse", "concourse", "cf"),
				Entry("when the previous lb type is cf", "cf", "concourse"),
			)

			Context("when availability zone retriever fails", func() {
				It("returns an error", func() {
					availabilityZoneRetriever.RetrieveCall.Returns.Error = errors.New("failed to retrieve azs")

					err := command.Execute(commands.AWSCreateLBsConfig{
						LBType:   "concourse",
						CertPath: "/path/to/cert",
						KeyPath:  "/path/to/key",
					}, storage.State{})
					Expect(err).To(MatchError("failed to retrieve azs"))
				})
			})

			Context("when update infrastructure manager fails", func() {
				It("returns an error", func() {
					infrastructureManager.UpdateCall.Returns.Error = errors.New("failed to update infrastructure")

					err := command.Execute(commands.AWSCreateLBsConfig{
						LBType:   "concourse",
						CertPath: "/path/to/cert",
						KeyPath:  "/path/to/key",
					}, storage.State{})
					Expect(err).To(MatchError("failed to update infrastructure"))
				})
			})

			Context("when lb is cf and cert path is invalid", func() {
				It("returns an error", func() {
					err := command.Execute(commands.AWSCreateLBsConfig{
						LBType:   "cf",
						CertPath: "/fake/cert/path",
						KeyPath:  "/some/key/path",
					}, storage.State{
						TFState: "some-tf-state",
					})
					Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
				})
			})

			Context("when lb is cf and key path is invalid", func() {
				var certPath string

				BeforeEach(func() {
					tempCertFile, err := ioutil.TempFile("", "cert")
					Expect(err).NotTo(HaveOccurred())
					certPath = tempCertFile.Name()
				})

				It("returns an error", func() {
					err := command.Execute(commands.AWSCreateLBsConfig{
						LBType:   "cf",
						CertPath: certPath,
						KeyPath:  "/fake/key/path",
					}, storage.State{
						TFState: "some-tf-state",
					})
					Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
				})
			})

			Context("when lb is cf and chain path is invalid", func() {
				var (
					certPath string
					keyPath  string
				)

				BeforeEach(func() {
					tempCertFile, err := ioutil.TempFile("", "cert")
					Expect(err).NotTo(HaveOccurred())
					certPath = tempCertFile.Name()

					tempKeyFile, err := ioutil.TempFile("", "key")
					Expect(err).NotTo(HaveOccurred())
					keyPath = tempKeyFile.Name()
				})

				It("returns an error", func() {
					err := command.Execute(commands.AWSCreateLBsConfig{
						LBType:    "cf",
						CertPath:  certPath,
						KeyPath:   keyPath,
						ChainPath: "/fake/chain/path",
					}, storage.State{
						TFState: "some-tf-state",
					})
					Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
				})
			})

			Context("when terraform manager fails to apply", func() {
				var (
					certPath string
					keyPath  string
				)

				BeforeEach(func() {
					tempCertFile, err := ioutil.TempFile("", "cert")
					Expect(err).NotTo(HaveOccurred())
					certPath = tempCertFile.Name()

					tempKeyFile, err := ioutil.TempFile("", "key")
					Expect(err).NotTo(HaveOccurred())
					keyPath = tempKeyFile.Name()
				})

				It("returns an error", func() {
					terraformManager.ApplyCall.Returns.Error = errors.New("failed to apply")

					err := command.Execute(commands.AWSCreateLBsConfig{
						LBType:   "concourse",
						CertPath: certPath,
						KeyPath:  keyPath,
					}, storage.State{
						TFState: "some-tf-state",
					})
					Expect(err).To(MatchError("failed to apply"))
				})
			})

			Context("when the terraform manager fails with terraformManagerError", func() {
				var (
					certPath string
					keyPath  string

					managerError *fakes.TerraformManagerError
				)

				BeforeEach(func() {
					tempCertFile, err := ioutil.TempFile("", "cert")
					Expect(err).NotTo(HaveOccurred())
					certPath = tempCertFile.Name()

					tempKeyFile, err := ioutil.TempFile("", "key")
					Expect(err).NotTo(HaveOccurred())
					keyPath = tempKeyFile.Name()

					managerError = &fakes.TerraformManagerError{}
					managerError.BBLStateCall.Returns.BBLState = storage.State{
						TFState: "some-partial-tf-state",
					}
					managerError.ErrorCall.Returns = "cannot apply"
					terraformManager.ApplyCall.Returns.Error = managerError
				})

				It("saves the bbl state and returns the error", func() {
					err := command.Execute(commands.AWSCreateLBsConfig{
						LBType:   "concourse",
						CertPath: certPath,
						KeyPath:  keyPath,
					}, storage.State{
						TFState: "some-tf-state",
					})
					Expect(err).To(MatchError("cannot apply"))

					Expect(stateStore.SetCall.CallCount).To(Equal(1))
					Expect(stateStore.SetCall.Receives[0].State).To(Equal(storage.State{
						TFState: "some-partial-tf-state",
					}))
				})

				Context("when the terraform manager error fails to return a bbl state", func() {
					BeforeEach(func() {
						managerError.BBLStateCall.Returns.Error = errors.New("failed to retrieve bbl state")
					})

					It("saves the bbl state and returns the error", func() {
						err := command.Execute(commands.AWSCreateLBsConfig{
							LBType:   "concourse",
							CertPath: certPath,
							KeyPath:  keyPath,
						}, storage.State{
							TFState: "some-tf-state",
						})
						Expect(err).To(MatchError("the following errors occurred:\ncannot apply,\nfailed to retrieve bbl state"))
					})
				})

				Context("when we fail to set the bbl state", func() {
					BeforeEach(func() {
						managerError.BBLStateCall.Returns.BBLState = storage.State{
							TFState: "some-partial-tf-state",
						}
						stateStore.SetCall.Returns = []fakes.SetCallReturn{
							{errors.New("failed to set bbl state")},
						}
					})

					It("saves the bbl state and returns the error", func() {
						err := command.Execute(commands.AWSCreateLBsConfig{
							LBType:   "concourse",
							CertPath: certPath,
							KeyPath:  keyPath,
						}, storage.State{
							TFState: "some-tf-state",
						})
						Expect(err).To(MatchError("the following errors occurred:\ncannot apply,\nfailed to set bbl state"))
					})
				})
			})

			Context("when certificate manager fails to create a certificate", func() {
				It("returns an error", func() {
					certificateManager.CreateCall.Returns.Error = errors.New("failed to create cert")

					err := command.Execute(commands.AWSCreateLBsConfig{
						LBType:   "concourse",
						CertPath: "/path/to/cert",
						KeyPath:  "/path/to/key",
					}, storage.State{})
					Expect(err).To(MatchError("failed to create cert"))
				})
			})

			Context("when cloud config manager update fails", func() {
				It("returns an error", func() {
					cloudConfigManager.UpdateCall.Returns.Error = errors.New("failed to update cloud config")

					err := command.Execute(commands.AWSCreateLBsConfig{
						LBType:   "concourse",
						CertPath: "/path/to/cert",
						KeyPath:  "/path/to/key",
					}, storage.State{})
					Expect(err).To(MatchError("failed to update cloud config"))
				})
			})

			It("returns an error when a GUID cannot be generated", func() {
				guidGenerator.GenerateCall.Returns.Error = errors.New("Out of entropy in the universe")
				err := command.Execute(commands.AWSCreateLBsConfig{
					LBType:   "concourse",
					CertPath: "/path/to/cert",
					KeyPath:  "/path/to/key",
				}, storage.State{})
				Expect(err).To(MatchError("Out of entropy in the universe"))
			})

			It("returns an error when the state fails to save", func() {
				stateStore.SetCall.Returns = []fakes.SetCallReturn{{errors.New("failed to save state")}}
				err := command.Execute(commands.AWSCreateLBsConfig{
					LBType:   "concourse",
					CertPath: "/path/to/cert",
					KeyPath:  "/path/to/key",
				}, storage.State{})
				Expect(err).To(MatchError("failed to save state"))
			})
		})
	})
})
