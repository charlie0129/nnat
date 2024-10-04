/*
Copyright 2023 Charlie Chiang

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

package main

import (
	"fmt"

	"github.com/fatih/color"

	"github.com/charlie0129/template-go/pkg/version"
)

func main() {
	name := color.New(color.Bold).Sprint("nnatc")
	ver := color.New(color.FgGreen).Sprint(version.Version)
	commit := color.New(color.FgBlue).Sprint(version.GitCommit)
	fmt.Printf("This is %s version %s commit %s.\n", name, ver, commit)
}
