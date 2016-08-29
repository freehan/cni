// Copyright 2016 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"github.com/containernetworking/cni/pkg/types"
	fakestore "github.com/containernetworking/cni/plugins/ipam/host-local/backend/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net"
)

type AllocatorTestCase struct {
	subnet       string
	ipmap        map[string]string
	expectResult string
	lastIP       string
}

func (t AllocatorTestCase) run() (*types.IPConfig, error) {
	subnet, err := types.ParseCIDR(t.subnet)
	if err != nil {
		return nil, err
	}

	conf := IPAMConfig{
		Name:   "test",
		Type:   "host-local",
		Subnet: types.IPNet{IP: subnet.IP, Mask: subnet.Mask},
	}
	store := fakestore.NewFakeStore(t.ipmap, net.ParseIP(t.lastIP))
	alloc, err := NewIPAllocator(&conf, store)
	if err != nil {
		return nil, err
	}
	res, err := alloc.Get("ID")
	if err != nil {
		return nil, err
	}

	return res, nil
}

var _ = Describe("host-local ip allocator", func() {
	Context("when has free ip", func() {
		It("should allocate ips in round robin", func() {
			testCases := []AllocatorTestCase{
				// fresh start
				{
					subnet:       "10.0.0.0/29",
					ipmap:        map[string]string{},
					expectResult: "10.0.0.2",
					lastIP:       "",
				},
				{
					subnet: "10.0.0.0/29",
					ipmap: map[string]string{
						"10.0.0.2": "id",
					},
					expectResult: "10.0.0.3",
					lastIP:       "",
				},
				// wrap around
				{
					subnet:       "10.0.0.0/29",
					ipmap:        map[string]string{},
					expectResult: "10.0.0.2",
					lastIP:       "10.0.0.6",
				},
				// next ip of last reserved ip
				{
					subnet: "10.0.0.0/29",
					ipmap: map[string]string{
						"10.0.0.2": "id",
						"10.0.0.3": "id",
						"10.0.0.4": "id",
					},
					expectResult: "10.0.0.6",
					lastIP:       "10.0.0.5",
				},
				// round robin to the beginning
				{
					subnet: "10.0.0.0/29",
					ipmap: map[string]string{
						"10.0.0.5": "id",
						"10.0.0.6": "id",
					},
					expectResult: "10.0.0.2",
					lastIP:       "10.0.0.4",
				},
				// lastIP is out of range
				{
					subnet: "10.0.0.0/29",
					ipmap: map[string]string{
						"10.0.0.2": "id",
					},
					expectResult: "10.0.0.3",
					lastIP:       "10.0.0.128",
				},
				// lastIP is subnet broadcast address
				{
					subnet: "10.0.0.0/29",
					ipmap: map[string]string{
						"10.0.0.2": "id",
					},
					expectResult: "10.0.0.3",
					lastIP:       "10.0.0.7",
				},
			}

			for _, tc := range testCases {
				res, err := tc.run()
				Expect(err).ToNot(HaveOccurred())
				Expect(res.IP.IP.String()).To(Equal(tc.expectResult))
			}
		})

		Context("when requesting a specific IP", func() {
			It("must allocate the requested IP", func() {
				subnet, err := types.ParseCIDR("10.0.0.0/29")
				Expect(err).ToNot(HaveOccurred())
				requestedIP := net.ParseIP("10.0.0.2")
				ipmap := map[string]string{}
				conf := IPAMConfig{
					Name:   "test",
					Type:   "host-local",
					Subnet: types.IPNet{IP: subnet.IP, Mask: subnet.Mask},
					Args:   &IPAMArgs{IP: requestedIP},
				}
				store := fakestore.NewFakeStore(ipmap, nil)
				alloc, _ := NewIPAllocator(&conf, store)
				res, err := alloc.Get("ID")
				Expect(err).ToNot(HaveOccurred())
				Expect(res.IP.IP.String()).To(Equal(requestedIP.String()))
			})
		})
	})

	Context("when out of ips", func() {
		It("returns a meaningful error", func() {
			testCases := []AllocatorTestCase{
				{
					subnet: "10.0.0.0/30",
					ipmap: map[string]string{
						"10.0.0.2": "id",
					},
				},
				{
					subnet: "10.0.0.0/29",
					ipmap: map[string]string{
						"10.0.0.2": "id",
						"10.0.0.3": "id",
						"10.0.0.4": "id",
						"10.0.0.5": "id",
						"10.0.0.6": "id",
					},
				},
			}
			for _, tc := range testCases {
				_, err := tc.run()
				Expect(err).To(MatchError("no IP addresses available in network: test"))
			}
		})
	})
})
