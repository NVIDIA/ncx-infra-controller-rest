/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package carbidecli

import (
	"fmt"

	cli "github.com/urfave/cli/v2"
)

// NewApp builds a cli.App from the embedded OpenAPI spec data.
func NewApp(specData []byte) (*cli.App, error) {
	spec, err := ParseSpec(specData)
	if err != nil {
		return nil, fmt.Errorf("parsing embedded spec: %w", err)
	}

	defaultBaseURL := ""
	if len(spec.Servers) > 0 {
		defaultBaseURL = spec.Servers[0].URL
	}

	commands := BuildCommands(spec)
	commands = append(commands, LoginCommand())
	commands = append(commands, InitCommand())
	commands = append(commands, completionCommand())

	app := &cli.App{
		Name:                 "carbidecli",
		Usage:                spec.Info.Title,
		Version:              spec.Info.Version,
		EnableBashCompletion: true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Usage:   "Path to config file",
				EnvVars: []string{"CARBIDE_CONFIG"},
			},
			&cli.StringFlag{
				Name:    "base-url",
				Usage:   "API base URL",
				EnvVars: []string{"CARBIDE_BASE_URL"},
				Value:   defaultBaseURL,
			},
			&cli.StringFlag{
				Name:    "org",
				Usage:   "Organization name",
				EnvVars: []string{"CARBIDE_ORG"},
			},
			&cli.StringFlag{
				Name:    "token",
				Usage:   "API bearer token",
				EnvVars: []string{"CARBIDE_TOKEN"},
			},
			&cli.StringFlag{
				Name:  "token-command",
				Usage: "Shell command that prints a bearer token",
			},
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "Enable debug logging",
			},
			&cli.StringFlag{
				Name:    "token-url",
				Usage:   "OIDC token endpoint URL for login and token refresh",
				EnvVars: []string{"CARBIDE_TOKEN_URL"},
			},
			&cli.StringFlag{
				Name:    "keycloak-url",
				Usage:   "Keycloak base URL (constructs token-url if --token-url is not set)",
				EnvVars: []string{"CARBIDE_KEYCLOAK_URL"},
			},
			&cli.StringFlag{
				Name:    "keycloak-realm",
				Usage:   "Keycloak realm (used with --keycloak-url)",
				EnvVars: []string{"CARBIDE_KEYCLOAK_REALM"},
				Value:   "carbide-dev",
			},
			&cli.StringFlag{
				Name:    "client-id",
				Usage:   "OAuth client ID",
				EnvVars: []string{"CARBIDE_CLIENT_ID"},
				Value:   "carbide-api",
			},
		},
		Commands: commands,
		Before: func(c *cli.Context) error {
			if cfg := c.String("config"); cfg != "" {
				SetConfigPath(cfg)
			}
			return nil
		},
	}

	return app, nil
}

func completionCommand() *cli.Command {
	return &cli.Command{
		Name:  "completion",
		Usage: "Output shell completion script",
		Subcommands: []*cli.Command{
			{
				Name:  "bash",
				Usage: "Output bash completion script",
				Action: func(c *cli.Context) error {
					fmt.Print(bashCompletion)
					return nil
				},
			},
			{
				Name:  "zsh",
				Usage: "Output zsh completion script",
				Action: func(c *cli.Context) error {
					fmt.Print(zshCompletion)
					return nil
				},
			},
			{
				Name:  "fish",
				Usage: "Output fish completion script",
				Action: func(c *cli.Context) error {
					fmt.Print(fishCompletion)
					return nil
				},
			},
		},
	}
}

const bashCompletion = `# bash completion for carbidecli
# Add to ~/.bashrc:  eval "$(carbidecli completion bash)"
_carbidecli_complete() {
    local cur opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    opts=$(${COMP_WORDS[0]} --generate-bash-completion "${COMP_WORDS[@]:1:$COMP_CWORD}")
    COMPREPLY=($(compgen -W "${opts}" -- "${cur}"))
    return 0
}
complete -o default -F _carbidecli_complete carbidecli
`

const zshCompletion = `# zsh completion for carbidecli
# Add to ~/.zshrc:  eval "$(carbidecli completion zsh)"
_carbidecli_complete() {
    local -a opts
    opts=(${(f)"$(${words[1]} --generate-bash-completion ${words:1:$CURRENT-1})"})
    _describe 'carbidecli' opts
}
compdef _carbidecli_complete carbidecli
`

const fishCompletion = `# fish completion for carbidecli
# Add to ~/.config/fish/completions/carbidecli.fish or run:
#   carbidecli completion fish > ~/.config/fish/completions/carbidecli.fish
complete -c carbidecli -f -a '(carbidecli --generate-bash-completion (commandline -cop))'
`
