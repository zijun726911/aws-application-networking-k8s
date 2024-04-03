package integration

import (
	"fmt"
	"log"
	 "os"
	"time"

 	"github.com/aws/aws-sdk-go/service/vpclattice"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	//"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/aws-application-networking-k8s/pkg/model/core"
	"github.com/aws/aws-application-networking-k8s/test/pkg/test"
    "sigs.k8s.io/gateway-api/apis/v1alpha2"
    "sigs.k8s.io/gateway-api/apis/v1beta1"
    gwv1 "sigs.k8s.io/gateway-api/apis/v1"
)

var _ = Describe("TLSRoute test", func() {
	var (
		deployment1        *appsv1.Deployment
		service1           *v1.Service
		tlsRoute *v1alpha2.TLSRoute
	)

	It("create a tlsRoute", func() {
		deployment1, service1 = testFramework.NewHttpsApp(test.HTTPsAppOptions{Name: "my-https-1", Namespace: k8snamespace})
		tlsRoute = testFramework.NewTLSRoute(k8snamespace, testGateway, []v1alpha2.TLSRouteRule {
            {
               BackendRefs: []gwv1.BackendRef {
                  {
                                BackendObjectReference: v1beta1.BackendObjectReference{
                                    Name:      v1alpha2.ObjectName(service1.Name),
                                    Namespace: lo.ToPtr(v1beta1.Namespace(service1.Namespace)),
                                    Kind:      lo.ToPtr(v1beta1.Kind("Service")),
                                    Port:      lo.ToPtr(v1beta1.PortNumber(443)),
                                },
                },
              },
            },
         })
 

		// Create Kubernetes API Objects
		testFramework.ExpectCreated(ctx,
			tlsRoute,
			service1,
			deployment1,
		)
		route, _ := core.NewRoute(tlsRoute)
		vpcLatticeService := testFramework.GetVpcLatticeService(ctx, route)
		fmt.Printf("vpcLatticeService: %v \n", vpcLatticeService)

		targetGroupV1 := testFramework.GetTCPTargetGroup(ctx, service1)
		Expect(*targetGroupV1.VpcIdentifier).To(Equal(os.Getenv("CLUSTER_VPC_ID")))
		Expect(*targetGroupV1.Protocol).To(Equal("TCP"))
		targetsV1 := testFramework.GetTargets(ctx, targetGroupV1, deployment1)
		Expect(*targetGroupV1.Port).To(BeEquivalentTo(80))
		for _, target := range targetsV1 {
			Expect(*target.Port).To(BeEquivalentTo(service1.Spec.Ports[0].TargetPort.IntVal))
			Expect(*target.Status).To(Or(
				Equal(vpclattice.TargetStatusInitial),
				Equal(vpclattice.TargetStatusHealthy),
			))
		}
/*
		log.Println("Verifying VPC lattice service listeners and rules")
		Eventually(func(g Gomega) {
			listListenerResp, err := testFramework.LatticeClient.ListListenersWithContext(ctx, &vpclattice.ListListenersInput{
				ServiceIdentifier: vpcLatticeService.Id,
			})
			g.Expect(err).To(BeNil())
			g.Expect(len(listListenerResp.Items)).To(BeEquivalentTo(1))
			listener := listListenerResp.Items[0]
			g.Expect(*listener.Port).To(BeEquivalentTo(testGateway.Spec.Listeners[0].Port))
			listenerId := listener.Id
			listRulesResp, err := testFramework.LatticeClient.ListRulesWithContext(ctx, &vpclattice.ListRulesInput{
				ListenerIdentifier: listenerId,
				ServiceIdentifier:  vpcLatticeService.Id,
			})
			nonDefaultRules := lo.Filter(listRulesResp.Items, func(rule *vpclattice.RuleSummary, _ int) bool {
				return rule.IsDefault == nil || *rule.IsDefault == false
			})
			ruleIds := lo.Map(nonDefaultRules, func(rule *vpclattice.RuleSummary, _ int) *string {
				return rule.Id
			})

			g.Expect(len(ruleIds)).To(Equal(2))

			rule0, err := testFramework.LatticeClient.GetRuleWithContext(ctx, &vpclattice.GetRuleInput{
				ServiceIdentifier:  vpcLatticeService.Id,
				ListenerIdentifier: listenerId,
				RuleIdentifier:     ruleIds[0],
			})
			g.Expect(err).To(BeNil())

			rule1, err := testFramework.LatticeClient.GetRuleWithContext(ctx, &vpclattice.GetRuleInput{
				ServiceIdentifier:  vpcLatticeService.Id,
				ListenerIdentifier: listenerId,
				RuleIdentifier:     ruleIds[1],
			})
			tlsrouteRules := tlsRoute.Spec.Rules

   fmt.Printf("rule0 = %v, rule1 = %v tlsrouteRules = %v \n", rule0, rule1, tlsrouteRules)

			g.Expect(err).To(BeNil())

			retrievedRules := []string{
				*rule0.Match.HttpMatch.PathMatch.Match.Prefix,
				*rule1.Match.HttpMatch.PathMatch.Match.Prefix}
			expectedRules := []string{*httprouteRules[0].Matches[0].Path.Value,
				*httprouteRules[1].Matches[0].Path.Value}
			log.Println("retrievedRules", retrievedRules)
			log.Println("expectedRules", expectedRules)

			g.Expect(retrievedRules).To(
				ContainElements(expectedRules))

		}).WithOffset(1).Should(Succeed())
*/
		log.Println("Verifying traffic")
        log.Println(">>>>>>>>>>>>>> Liwen Wu >>>>>>>>>>>>>>>>>>>>")
		time.Sleep(900 * time.Second)
		dnsName := testFramework.GetVpcLatticeServiceDns(tlsRoute.Name, tlsRoute.Namespace)

		testFramework.Get(ctx, types.NamespacedName{Name: deployment1.Name, Namespace: deployment1.Namespace}, deployment1)

		//get the pods of deployment1
		pods := testFramework.GetPodsByDeploymentName(deployment1.Name, deployment1.Namespace)
		Expect(len(pods)).To(BeEquivalentTo(1))
		pod := pods[0]

		Eventually(func(g Gomega) {
			cmd := fmt.Sprintf("curl %s/pathmatch0", dnsName)
			stdout, _, err := testFramework.PodExec(pod, cmd)
			g.Expect(err).To(BeNil())
			g.Expect(stdout).To(ContainSubstring("test-v1 handler pod"))
		}).WithTimeout(30 * time.Second).WithOffset(1).Should(Succeed())

	})

	AfterEach(func() {
		testFramework.ExpectDeletedThenNotFound(ctx,
			//tlsRoute,
			deployment1,
			service1,
		)
	})
})

