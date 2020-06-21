/*
Copyright 2020 The Operator-SDK Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package manager

import "math/rand"

//todo: see that it is new and need to be pushed for SDK lib
func randomString(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}

// GenRandomLeaderElectionID will return an string to be used as LeaderElectionID (E.g BpLnfgDsc2.helm.operator-sdk)
// This method is required for Helm/Ansible based-operators using the new layout when the flag with
// the leader-election-id is not informed.
func GenRandomLeaderElectionID(value string) string {
	return randomString(8) + value
}


