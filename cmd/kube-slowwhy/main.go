package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/marek-kar/kube-slowwhy/pkg/collector"
)

func main() {
	root := &cobra.Command{
		Use:   "kube-slowwhy",
		Short: "Diagnose slow Kubernetes clusters",
	}

	root.AddCommand(newCollectCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func newCollectCmd() *cobra.Command {
	opts := collector.DefaultOptions()
	var since string

	cmd := &cobra.Command{
		Use:   "collect",
		Short: "Collect a cluster snapshot for offline analysis",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := time.ParseDuration(since)
			if err != nil {
				return fmt.Errorf("invalid --since value: %w", err)
			}
			opts.Since = d

			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
			defer cancel()

			client, err := buildClient()
			if err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Collecting cluster snapshot (since %s)...\n", opts.Since)
			snap, err := collector.Collect(ctx, client, opts)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
			}
			if snap == nil {
				return fmt.Errorf("snapshot is nil")
			}

			data, err := json.MarshalIndent(snap, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal snapshot: %w", err)
			}

			if err := os.WriteFile(opts.Output, data, 0o644); err != nil {
				return fmt.Errorf("write file: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Snapshot written to %s\n", opts.Output)
			return nil
		},
	}

	cmd.Flags().StringVar(&since, "since", "30m", "look-back duration for events")
	cmd.Flags().StringVarP(&opts.Namespace, "namespace", "n", "", "filter by namespace (empty = all)")
	cmd.Flags().StringVarP(&opts.Output, "out", "o", opts.Output, "output file path")

	return cmd
}

func buildClient() (kubernetes.Interface, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		rules, &clientcmd.ConfigOverrides{},
	).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}
	return client, nil
}
