package cmd

import (
	"bytes"
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"

	"github.com/JeffreyFalgout/cron2mqtt/mqtt"
)

func init() {
	viper.SetConfigName("cron2mqtt")
	viper.SetConfigType("json")
	viper.AddConfigPath("$HOME/.config")

	var broker, username, serverName string
	var setPassword bool
	configure := &cobra.Command{
		Use:   "configure",
		Short: "Configures how this tool publishes to MQTT.",
		Long:  "The configuration will be written to a per-user config file. This is to prevent passwords from being more visible than strictly necessary.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			if broker == "" && username == "" && serverName == "" && !setPassword {
				return fmt.Errorf("configure must be called with at least one of its flags")
			}

			if setPassword {
				pwd, err := promptPassword()
				if err != nil {
					return err
				}
				viper.Set("password", string(pwd))
			}

			_, err := loadConfig()
			if err != nil {
				if _, ok := err.(viper.ConfigFileNotFoundError); ok {
					// OK
				} else {
					fmt.Fprintf(os.Stderr, "Existing config appears to be corrupt: %s\n", err)
				}
			}

			f := viper.ConfigFileUsed()
			save := viper.SafeWriteConfig
			chmod := true
			if f != "" {
				save = viper.WriteConfig
				chmod = false
			}

			if err := save(); err != nil {
				return fmt.Errorf("failed to write config file: %w", err)
			}

			if err := viper.ReadInConfig(); err != nil {
				return fmt.Errorf("failed to write config file: %w", err)
			}
			f = viper.ConfigFileUsed()
			fmt.Printf("Wrote config to %s\n", f)
			if chmod {
				if err := os.Chmod(f, 0600); err != nil {
					return fmt.Errorf("failed to update permissions on %s: %w", f, err)
				}
			}
			return nil
		},
	}
	configure.Flags().StringVar(&broker, "broker", "", "The broker to connect to. Should be of the form scheme://host:port where scheme is one of tcp, ssl, ws.")
	configure.Flags().StringVar(&username, "username", "", "The username to use when connecting to the broker.")
	configure.Flags().BoolVar(&setPassword, "password", false, "Indicates that you want to configure the password used to connect to the broker. You will be prompted to enter the password through stdin.")
	configure.Flags().StringVar(&serverName, "server_name", "", "Overrides the broker's host name when doing ssl verification.")

	viper.BindPFlag("broker", configure.Flags().Lookup("broker"))
	viper.BindPFlag("username", configure.Flags().Lookup("username"))
	viper.BindPFlag("server_name", configure.Flags().Lookup("server_name"))

	rootCmd.AddCommand(configure)
}

func promptPassword() ([]byte, error) {
	fmt.Println("NOTE: Passwords are sent without any additional encryption. It's strongly recommended that you use ssl:// so that passwords don't show up as plaintext to everyone on your network.")
	for {
		fmt.Print("Enter password: ")
		pwd, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return nil, err
		}

		fmt.Print("Confirm password: ")
		pwd2, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return nil, err
		}

		if bytes.Equal(pwd, pwd2) {
			return pwd, nil
		}

		fmt.Println("Passwords did not match. Try again.")
	}
}

func loadConfig() (mqtt.Config, error) {
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return mqtt.Config{}, err
		}
		return mqtt.Config{}, fmt.Errorf("error reading config %s: %w", viper.ConfigFileUsed(), err)
	}

	var c mqtt.Config
	if err := viper.Unmarshal(&c); err != nil {
		return mqtt.Config{}, err
	}
	return c, nil
}
