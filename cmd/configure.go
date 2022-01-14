package cmd

import (
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

	var setPassword bool
	configure := &cobra.Command{
		Use:   "configure",
		Short: "Configures how this tool publishes to MQTT.",
		Long:  "The configuration will be written to a per-user config file. This is to prevent passwords from being more visible than strictly necessary.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			if setPassword {
				fmt.Println("NOTE: Passwords are sent without any additional encryption. It's strongly recommended that you use ssl:// so that passwords don't show up as plaintext to everyone on your network.")
				fmt.Print("Enter password: ")
				pwd, err := term.ReadPassword(int(syscall.Stdin))
				if err != nil {
					return err
				}
				fmt.Println()
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
	configure.Flags().String("broker", "", "The broker to connect to. Should be of the form scheme://host:port where scheme is one of tcp, ssl, ws.")
	configure.Flags().String("username", "", "The username to use when connecting to the broker.")
	configure.Flags().BoolVar(&setPassword, "password", false, "Indicates that you want to configure the password used to connect to the broker. You will be prompted to enter the password through stdin.")
	configure.Flags().String("server_name", "", "Overrides the broker's host name when doing ssl verification.")

	viper.BindPFlag("broker", configure.Flags().Lookup("broker"))
	viper.BindPFlag("username", configure.Flags().Lookup("username"))
	viper.BindPFlag("server_name", configure.Flags().Lookup("server_name"))

	rootCmd.AddCommand(configure)
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
