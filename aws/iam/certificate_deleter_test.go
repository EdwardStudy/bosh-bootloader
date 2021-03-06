package iam_test

import (
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	awsiam "github.com/aws/aws-sdk-go/service/iam"
	"github.com/cloudfoundry/bosh-bootloader/aws/iam"
	"github.com/cloudfoundry/bosh-bootloader/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CertificateDeleter", func() {
	var (
		iamClient         *fakes.IAMClient
		awsClientProvider *fakes.AWSClientProvider
		deleter           iam.CertificateDeleter
	)

	BeforeEach(func() {
		iamClient = &fakes.IAMClient{}
		awsClientProvider = &fakes.AWSClientProvider{}
		awsClientProvider.GetIAMClientCall.Returns.IAMClient = iamClient

		deleter = iam.NewCertificateDeleter(awsClientProvider)
	})

	Describe("Delete", func() {
		It("deletes the certificates with the given name", func() {
			iamClient.DeleteServerCertificateCall.Returns.Output = &awsiam.DeleteServerCertificateOutput{}

			err := deleter.Delete("some-certificate")
			Expect(err).NotTo(HaveOccurred())

			Expect(iamClient.DeleteServerCertificateCall.Receives.Input.ServerCertificateName).To(Equal(aws.String("some-certificate")))
		})

		Context("failure cases", func() {
			It("returns an error when it fails to delete", func() {
				iamClient.DeleteServerCertificateCall.Returns.Error = errors.New("failed to delete certificate")

				err := deleter.Delete("some-certificate")
				Expect(err).To(MatchError("failed to delete certificate"))
			})
		})
	})
})
